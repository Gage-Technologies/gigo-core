package follower

import (
	"bytes"
	"fmt"
	"github.com/sourcegraph/conc/pool"
	"net/url"
	"time"

	"context"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/gage-technologies/gigo-lib/storage"
	"github.com/kisielk/sqlstruct"
	"github.com/nats-io/nats.go"
	"github.com/sabloger/sitemap-generator/smg"
	"go.opentelemetry.io/otel"
)

func GenerateSiteMap(ctx context.Context, db *ti.Database, js *mq.JetstreamClient, storageEngine storage.Storage, nodeId int64, host string, workerPool *pool.Pool, logger logging.Logger) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "generate-site-map-routine")
	defer parentSpan.End()
	callerName := "GenerateSiteMap"

	// create subscription for session key management
	_, err := js.ConsumerInfo(streams.SubjectMiscSitemapGenerate, "gigo-core-follower-sitemap-generation")
	if err != nil {
		_, err = js.AddConsumer(streams.StreamMisc, &nats.ConsumerConfig{
			Durable:       "gigo-core-follower-sitemap-generation",
			AckPolicy:     nats.AckExplicitPolicy,
			AckWait:       time.Second * 30,
			FilterSubject: streams.SubjectMiscSitemapGenerate,
		})
		if err != nil {
			logger.Errorf("(sitemap_gen: %d) failed to create sitemap gen consumer: %v", nodeId, err)
			return
		}
	}
	sub, err := js.PullSubscribe(streams.SubjectMiscSitemapGenerate, "gigo-core-follower-sitemap-generation", nats.AckExplicit())
	if err != nil {
		logger.Errorf("(sitemap_gen: %d) failed to create sitemap gen subscription: %v", nodeId, err)
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
		logger.Errorf("(sitemap_gen: %d) failed to retrieve message from jetstream: %v", nodeId, err)
		return
	}

	workerPool.Go(func() {
		logger.Debugf("(sitemap_gen: %d) beginning sitemap generation job", nodeId)

		// defer the message ack
		defer msg.Ack()

		n := time.Now()

		sm := smg.NewSitemap(true)
		sm.SetName("GIGO Sitemap")
		sm.SetHostname("https://www.gigo.dev")
		sm.SetLastMod(&n)
		sm.SetCompress(true)

		err = sm.Add(&smg.SitemapLoc{
			Loc:        "/",
			LastMod:    &n,
			ChangeFreq: smg.Daily,
			Priority:   0.4,
		})
		if err != nil {
			logger.Errorf("(sitemap_gen: %d) unable to add home to the sitemap: %v", nodeId, err)
			return
		}

		err = sm.Add(&smg.SitemapLoc{
			Loc:        "/about",
			LastMod:    &n,
			ChangeFreq: smg.Weekly,
			Priority:   0.4,
		})
		if err != nil {
			logger.Errorf("(sitemap_gen: %d) unable to add about to the sitemap: %v", nodeId, err)
			return
		}

		err = sm.Add(&smg.SitemapLoc{
			Loc:        "/aboutBytes",
			LastMod:    &n,
			ChangeFreq: smg.Weekly,
			Priority:   0.4,
		})
		if err != nil {
			logger.Errorf("(sitemap_gen: %d) unable to add about bytes to the sitemap: %v", nodeId, err)
			return
		}

		err = sm.Add(&smg.SitemapLoc{
			Loc:        "/documentation",
			LastMod:    &n,
			ChangeFreq: smg.Weekly,
			Priority:   0.4,
		})
		if err != nil {
			logger.Errorf("(sitemap_gen: %d) unable to add dcos to the sitemap: %v", nodeId, err)
			return
		}

		err = sm.Add(&smg.SitemapLoc{
			Loc:        "/premium",
			LastMod:    &n,
			ChangeFreq: smg.Weekly,
			Priority:   0.4,
		})
		if err != nil {
			logger.Errorf("(sitemap_gen: %d) unable to add premium to the sitemap: %v", nodeId, err)
			return
		}

		err = sm.Add(&smg.SitemapLoc{
			Loc:        "/buyingExclusive",
			LastMod:    &n,
			ChangeFreq: smg.Weekly,
			Priority:   0.4,
		})
		if err != nil {
			logger.Errorf("(sitemap_gen: %d) unable to add buyingExclusive to the sitemap: %v", nodeId, err)
			return
		}

		err = sm.Add(&smg.SitemapLoc{
			Loc:        "/aboutExclusive",
			LastMod:    &n,
			ChangeFreq: smg.Weekly,
			Priority:   0.4,
		})
		if err != nil {
			logger.Errorf("(sitemap_gen: %d) unable to add buyingExclusive to the sitemap: %v", nodeId, err)
			return
		}

		// query for all posts that are not deleted, unpublished or private
		res, err := db.Query(
			ctx, &parentSpan, &callerName,
			"select _id, updated_at from post where published=true and deleted=false and visibility not in (?, ?);",
			models.PrivateVisibility, models.ExclusiveVisibility,
		)
		if err != nil {
			logger.Errorf("(sitemap_gen: %d) unable to query for posts: %v", nodeId, err)
		}
		defer res.Close()

		postCount := 0
		for res.Next() {
			var loc smg.SitemapLoc
			var post models.PostSQL

			err = sqlstruct.Scan(&post, res)
			if err != nil {
				logger.Errorf("(sitemap_gen: %d) unable to parse post struct: %v", nodeId, err)
				return
			}

			loc.Loc = fmt.Sprintf("/challenge/%d", post.ID)
			loc.LastMod = &post.UpdatedAt
			loc.ChangeFreq = smg.Weekly
			loc.Priority = 0.4

			err = sm.Add(&loc)
			if err != nil {
				logger.Errorf("(sitemap_gen: %d) unable to add post to the sitemap: %v", nodeId, err)
				return
			}
			postCount++
		}

		logger.Infof("(sitemap_gen: %d) added %d posts to sitemap", nodeId, postCount)

		_ = res.Close()

		// query for all bytes that are published
		res, err = db.Query(
			ctx, &parentSpan, &callerName,
			"select _id from bytes where published=true;",
		)
		if err != nil {
			logger.Errorf("(sitemap_gen: %d) unable to query for bytes: %v", nodeId, err)
		}
		defer res.Close()

		byteCount := 0
		for res.Next() {
			var loc smg.SitemapLoc
			var id int64

			err = res.Scan(&id)
			if err != nil {
				logger.Errorf("(sitemap_gen: %d) unable to parse byte id: %v", nodeId, err)
				return
			}

			sfId := snowflake.ParseInt64(id)
			updatedAt := time.UnixMilli(sfId.Time())

			loc.Loc = fmt.Sprintf("/byte/%d", id)
			loc.LastMod = &updatedAt
			loc.ChangeFreq = smg.Weekly
			loc.Priority = 0.4

			err = sm.Add(&loc)
			if err != nil {
				logger.Errorf("(sitemap_gen: %d) unable to add byte to the sitemap: %v", nodeId, err)
				return
			}
			byteCount++
		}

		logger.Infof("(sitemap_gen: %d) added %d bytes to sitemap", nodeId, byteCount)

		_ = res.Close()

		// query for all users
		res, err = db.Query(
			ctx, &parentSpan, &callerName,
			"select user_name, created_at from users where is_ephemeral = false",
		)
		if err != nil {
			logger.Errorf("(sitemap_gen: %d) unable to get posts from database: %v", nodeId, err)
			return
		}
		defer res.Close()

		userCount := 0
		for res.Next() {
			var loc smg.SitemapLoc
			var username string
			var createdAt time.Time

			err = res.Scan(&username, &createdAt)
			if err != nil {
				logger.Errorf("(sitemap_gen: %d) unable to parse post struct: %v", nodeId, err)
				return
			}

			loc.Loc = fmt.Sprintf("/user/%s", url.QueryEscape(username))
			loc.LastMod = &createdAt
			loc.ChangeFreq = smg.Weekly
			loc.Priority = 0.4

			err = sm.Add(&loc)
			if err != nil {
				logger.Errorf("(sitemap_gen: %d) unable to add user to the sitemap: %v", nodeId, err)
				continue
			}

			userCount++
		}

		logger.Infof("(sitemap_gen: %d) added %d users to sitemap", nodeId, userCount)

		_ = res.Close()

		// complete the sitemap
		sm.Finalize()

		// create a new buffer to write the sitemap to
		buffer := bytes.NewBuffer(nil)

		// write the sitemap to the buffer
		_, err = sm.WriteTo(buffer)
		if err != nil {
			logger.Errorf("(sitemap_gen: %d) unable to write sitemap to buffer: %v", nodeId, err)
			return
		}

		logger.Debugf("(sitemap_gen: %d) successfully wrote sitemap to buffer", nodeId)

		// save the sitemap to the storage engine
		err = storageEngine.CreateFile("sitemap/sitemap.xml", buffer.Bytes())
		if err != nil {
			logger.Errorf("(sitemap_gen: %d) unable to save sitemap to storage engine: %v", nodeId, err)
			return
		}

		logger.Infof("(sitemap_gen: %d) saved sitemap to storage engine", nodeId)
	})

	return
}
