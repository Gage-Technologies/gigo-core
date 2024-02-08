/*
 *  *********************************************************************************
 *   GAGE TECHNOLOGIES CONFIDENTIAL
 *   __________________
 *
 *    Gage Technologies
 *    Copyright (c) 2022
 *    All Rights Reserved.
 *
 *   NOTICE:  All information contained herein is, and remains
 *   the property of Gage Technologies and its suppliers,
 *   if any.  The intellectual and technical concepts contained
 *   herein are proprietary to Gage Technologies
 *   and its suppliers and may be covered by U.S. and Foreign Patents,
 *   patents in process, and are protected by trade secret or copyright law.
 *   Dissemination of this information or reproduction of this material
 *   is strictly forbidden unless prior written permission is obtained
 *   from Gage Technologies.
 */

package follower

import (
	"context"
	"fmt"
	"gigo-core/gigo/config"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/gage-technologies/gigo-lib/storage"
	"github.com/go-cmd/cmd"
	"github.com/nats-io/nats.go"
	"github.com/sourcegraph/conc/pool"
	"go.opentelemetry.io/otel"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var extractFullBackupTime = regexp.MustCompile("(?P<base_backup>_\\d+)(?P<inc_tag>_incr)?(?P<incr_backup>_\\d+)?")

func databaseBackupFull(cfg config.BackupConfig, db string, logger logging.Logger) error {
	// save start time of backup
	startTime := time.Now().UnixNano()

	// create variable to hold backup command
	var c *cmd.Cmd

	// assemble log path
	logPath := fmt.Sprintf("/tmp/db_backup_%s_%d.log", db, startTime)

	for _, store := range cfg.Stores {
		// create complete path to backup directory
		storagePath := fmt.Sprintf("%s_%d", filepath.Join(store.Bucket, "backups", db), startTime)

		// create function to clean up on failure
		cleanup := func() {
			// create s3 client for cleanup
			s3Client, err := storage.CreateMinioObjectStorage(store)
			if err != nil {
				logger.Error(fmt.Sprintf("failed to create s3 client for failed backup cleanup: %v", err))
				return
			}

			// delete the bucket
			err = s3Client.DeleteDir(storagePath, true)
			if err != nil {
				logger.Error(fmt.Sprintf("failed to delete failed backup: %v", err))
			}
		}

		logger.Debug(fmt.Sprintf("executing full backup command on s3 with @ %s - %s", store.Endpoint, "s3://"+storagePath))

		// format the endpoint
		endpoint := "http://" + store.Endpoint
		if store.UseSSL {
			endpoint = "https://" + store.Endpoint
		}

		// run the backup command
		c = cmd.NewCmd(
			"br",
			"backup",
			"db",
			"--db", db,
			"--pd", cfg.PD,
			"--storage", fmt.Sprintf("s3://%s?access-key=%s&secret-access-key=%s", storagePath, store.AccessKey, store.SecretKey),
			"--s3.endpoint", endpoint,
			"--s3.region", store.Region,
			"--ratelimit", "128",
			"--log-file", logPath,
			"--send-credentials-to-tikv=true",
		)

		// start cmd
		status := <-c.Start()
		stdout := ""
		if len(status.Stdout) > 0 {
			stdout = status.Stdout[len(status.Stdout)-1]
		}
		stderr := ""
		if len(status.Stderr) > 0 {
			stderr = status.Stderr[len(status.Stderr)-1]
		}

		if status.Exit != 0 || status.Error != nil {
			cleanup()
			return fmt.Errorf("failed to perform database backup\n    error: %v\n    status code: %d\n    log file: %s\n    stdout:\n%s\n    stderr:\n%s", status.Error, status.Exit, logPath, stdout, stderr)
		}

		logger.Debug(fmt.Sprintf("database backup command exited successfully\n    stdout\n%s\n    stderr:\n%s", stdout, stderr))
	}

	return nil
}

func cleanBackups(cfg config.BackupConfig, db string, logger logging.Logger) error {
	for _, store := range cfg.Stores {
		// create new s3 client
		s3Client, err := storage.CreateMinioObjectStorage(store)
		if err != nil {
			return fmt.Errorf("failed to create s3 client in backup clean operation: %v", err)
		}

		// retrieve all the backups
		allBackups, err := s3Client.ListDir("backups", true)
		if err != nil {
			return fmt.Errorf("failed to retrieve backup directories from s3 bucket: %v", err)
		}

		// filter backups for those corresponding to this database

		// create slice to hold directories
		backups := make([]string, 0)

		// filter for directories from this database
		for _, f := range allBackups {
			// skip if the backup is not for this database
			if !strings.HasPrefix(f, "backups/"+db) {
				continue
			}

			// extract backup info
			backupInfo := extractFullBackupTime.FindStringSubmatch(f)

			// exit if we didn't find the info
			if len(backupInfo) < 2 {
				logger.Warn(fmt.Sprintf("failed to extract prior backup info: %s", f))
				continue
			}

			// extract time of last full backup
			lastFullTime := strings.TrimPrefix(backupInfo[1], "_")

			// convert time to integer
			lastBackupTimeNanos, err := strconv.ParseInt(lastFullTime, 10, 64)
			if err != nil {
				logger.Error(fmt.Sprintf("failed to load last backup time to int: %v", f))
				continue
			}

			// ensure that the backup is ready from cleaning
			if time.Since(time.Unix(0, lastBackupTimeNanos)) < time.Hour*time.Duration(cfg.MaxBackupAgeHours) {
				continue
			}

			backups = append(backups, f)
		}

		// ensure there is at least the minimum amount of backups
		if len(backups) < cfg.MinRetainedBackups {
			logger.Debug("no backups found for cleaning using database: ", db)
			return nil
		}

		logger.Debug(fmt.Sprintf("%d prior backups found ready for cleaning\n%v", len(backups), backups))

		// iterate over each backup deleting them from storage
		for _, b := range backups {
			err = s3Client.DeleteDir(b, true)
			if err != nil {
				logger.Warn(fmt.Sprintf("failed to delete backup: %v", err))
			}
		}
	}
	return nil
}

func DatabaseBackupRoutine(ctx context.Context, cfg config.BackupConfig, js *mq.JetstreamClient, tidb *ti.Database, workerPool *pool.Pool, logger logging.Logger, nodeId int64) {
	if !cfg.Enabled {
		return
	}

	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "database-backup-routine")
	defer parentSpan.End()

	// create subscription for session key management
	_, err := js.ConsumerInfo(streams.SubjectDbBackupExec, "gigo-core-follower-database-backup")
	if err != nil {
		_, err = js.AddConsumer(streams.StreamDbBackup, &nats.ConsumerConfig{
			Durable:       "gigo-core-follower-database-backup",
			AckPolicy:     nats.AckExplicitPolicy,
			AckWait:       time.Second * 30,
			FilterSubject: streams.SubjectDbBackupExec,
		})
		if err != nil {
			logger.Errorf("(db_backup: %d) failed to create sitemap gen consumer: %v", nodeId, err)
			return
		}
	}
	sub, err := js.PullSubscribe(streams.SubjectDbBackupExec, "gigo-core-follower-database-backup", nats.AckExplicit())
	if err != nil {
		logger.Errorf("(db_backup: %d) failed to create db backup subscription: %v", nodeId, err)
		return
	}
	defer sub.Unsubscribe()

	// request next message from the subscriber. this tells use
	// that we are the follower to perform the key expiration.
	// we use such a short timeout because this will re-execute in
	// ~1s so if there is nothing to do now then we should not slow
	// down the refresh rate of the follower loop
	msg, err := getNextJob(sub, time.Millisecond*50)
	if err != nil {
		// exit silently for timeout because it simply means
		// there is nothing to do
		if err == context.DeadlineExceeded {
			return
		}
		logger.Errorf("(db_backup: %d) failed to retrieve message from jetstream: %v", nodeId, err)
		return
	}
	msg.Ack()

	workerPool.Go(func() {
		// log start of routine
		logger.Info("beginning database backup routine")
		start := time.Now()
		err = databaseBackupFull(cfg, tidb.DBName, logger)
		if err != nil {
			logger.Error(fmt.Sprintf("failed to perform full database backup: %v", err))
		} else {
			logger.Info(fmt.Sprintf("database backup completed successfully in %v", time.Since(start)))
			err = cleanBackups(cfg, tidb.DBName, logger)
			if err != nil {
				logger.Info(fmt.Sprintf("failed to perform backup cleaning: %v", err))
			} else {
				logger.Info("backup cleaning completed successfully")
			}
		}
	})
}
