package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc/credentials"

	"gigo-core/gigo/streak"

	"gigo-core/coder/api"
	"gigo-core/gigo/api/external_api"
	"gigo-core/gigo/api/ws"
	"gigo-core/gigo/config"
	"gigo-core/gigo/subroutines/follower"
	"gigo-core/gigo/subroutines/leader"
	"gigo-core/gigo/utils"

	"github.com/bwmarrin/snowflake"
	"github.com/gage-technologies/gigo-lib/cluster"
	config2 "github.com/gage-technologies/gigo-lib/config"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/search"
	"github.com/gage-technologies/gigo-lib/storage"
	"github.com/gage-technologies/gigo-lib/zitimesh"
	"github.com/go-redis/redis/v8"
	"github.com/sourcegraph/conc/pool"
	"github.com/stripe/stripe-go/v74"
	"github.com/syossan27/tebata"
	etcd "go.etcd.io/etcd/client/v3"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var (
	lock        = &sync.Mutex{}
	interrupted = false
)

func initTracer(insecure bool, collectorURL string, serviceName string) func(context.Context) error {
	secureOption := otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
	if insecure {
		secureOption = otlptracegrpc.WithInsecure()
	}

	exporter, err := otlptrace.New(
		context.Background(),
		otlptracegrpc.NewClient(
			secureOption,
			otlptracegrpc.WithEndpoint(collectorURL),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	resources, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("library.language", "go"),
		),
	)
	if err != nil {
		log.Fatal("Could not set resources: ", err)
	}

	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(resources),
		),
	)
	return exporter.Shutdown
}

func shutdown(server *external_api.HTTPServer, zitiManager *zitimesh.Manager, clusterNode cluster.Node,
	workerPool *pool.Pool, systemCancel context.CancelFunc, logger logging.Logger) {
	// we lock here so we can prevent the main thread from exiting
	// before we finish the graceful shutdown
	lock.Lock()
	defer lock.Unlock()

	// mark as interrupted
	interrupted = true

	// log that we received a shutdown request
	logger.Info("received termination - shutting down gracefully")
	fmt.Println("received termination - shutting down gracefully")

	// close server gracefully
	logger.Info("closing server")
	fmt.Println("closing server")
	err := server.Shutdown()
	if err != nil {
		logger.Errorf("failed to close server gracefully: %v", err)
		fmt.Printf("failed to close server gracefully: %v\n", err)
	}

	// close cluster node
	logger.Info("closing cluster node")
	fmt.Println("closing cluster node")
	clusterNode.Stop()
	err = clusterNode.Close()
	if err != nil {
		logger.Errorf("failed to close cluster node gracefully: %v", err)
		fmt.Printf("failed to close cluster node gracefully: %v\n", err)
	}

	// wait for all workers of the follower routine to exit
	logger.Info("waiting for follower workers to exit")
	fmt.Println("waiting for follower workers to exit")
	workerPool.Wait()

	// cancel system context
	logger.Info("canceling system context")
	fmt.Println("canceling system context")
	systemCancel()

	// delete ziti mesh node
	logger.Info("delete ziti server")
	fmt.Println("delete ziti server")
	zitiManager.DeleteServer(clusterNode.GetSelfMetadata().ID)

	logger.Info("server shutdown complete")
	fmt.Println("server shutdown complete")

	// flush logger so we get any last logs
	logger.Flush()
}

func main() {

	// set timezone to US Central
	err := os.Setenv("TZ", "America/Chicago")
	if err != nil {
		panic(err)
	}

	configPath := flag.String("configPath", "/home/user/Development/Projects/GIGO/src/gigo/config.yml", "Path to the configuration file for subroutines")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatal("failed to load config ", err)
	}

	cleanup := initTracer(cfg.OTELConfig.Insecure, cfg.OTELConfig.EndPoint, cfg.OTELConfig.ServiceName)
	defer cleanup(context.Background())

	// create snowflake node
	snowflakeNode, err := snowflake.NewNode(cfg.NodeID)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create snowflake node for gigo, %v", err))
	}

	// create a unique id for the node
	nodeID := snowflakeNode.Generate().Int64()

	fmt.Println("Node ID: ", nodeID)

	// update logger id to include the node id
	cfg.LoggerID = fmt.Sprintf("%s-%d", cfg.LoggerID, nodeID)

	// create system context
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	// parse access url
	parsedAccessUrl, err := url.Parse(cfg.AccessUrl)
	if err != nil {
		log.Fatal("failed to parse access url ", err)
	}

	// set global strip key
	stripe.Key = cfg.StripeKey

	fmt.Println("Creating logger")
	// create root logger
	rootLogger, err := logging.CreateESLogger(cfg.ESConfig.ESNodes, "alexis_system", cfg.ESConfig.ESPass, "gigo-core", cfg.LoggerID)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create es logger for gigo, %v", err))
	}

	// derive system  logger from root logger
	systemLogger := rootLogger.WithName("gigo-core-system")

	// create variable for storage engine
	var storageEngine storage.Storage

	fmt.Println("Creating storage engine")
	// initialize storage engine
	switch cfg.StorageConfig.Engine {
	case config2.StorageEngineS3:
		storageEngine, err = storage.CreateMinioObjectStorage(cfg.StorageConfig.S3)
		if err != nil {
			log.Fatalf("failed to create s3 object storage engine: %v", err)
		}
	case config2.StorageEngineFS:
		storageEngine, err = storage.CreateFileSystemStorage(cfg.StorageConfig.FS.Root)
		if err != nil {
			log.Fatalf("failed to create fs storage engine: %v", err)
		}
	default:
		log.Fatalf("invalid storage engine: %s", cfg.StorageConfig.Engine)
	}

	systemLogger.Infof("connecting to tidb @ %s:%s/%s", cfg.TitaniumConfig.TitaniumHost, cfg.TitaniumConfig.TitaniumPort, cfg.TitaniumConfig.TitaniumName)

	fmt.Println("Creating ti database")

	tiDB, err := ti.CreateDatabase(cfg.TitaniumConfig.TitaniumHost, cfg.TitaniumConfig.TitaniumPort, "mysql", cfg.TitaniumConfig.TitaniumUser,
		cfg.TitaniumConfig.TitaniumPassword, cfg.TitaniumConfig.TitaniumName)
	if err != nil {
		rootLogger.Errorf("failed to create titanium database: %v", err)
		rootLogger.Flush()
		log.Fatal("failed to create titanium database: ", err)
	}

	fmt.Println("Creating meili client")
	meili, err := search.CreateMeiliSearchEngine(cfg.MeiliConfig)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create meili search engine: %v", err))
	}

	// derive http logger from root logger
	httpLogger := rootLogger.WithName("gigo-core-external-api")

	// create variable to hold redis client
	var rdb redis.UniversalClient

	// ensure at least one node was passed for redis
	if len(cfg.RedisConfig.RedisNodes) < 1 {
		panic("redisNodes requires at least one node")
	}

	fmt.Println("Creating redis database")

	// create Redis connection by mapping to correct deployment config
	switch cfg.RedisConfig.RedisType {
	case "local":
		// create local client
		rdb = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisConfig.RedisNodes[0],
			Password: cfg.RedisConfig.RedisPassword,
			DB:       cfg.RedisConfig.RedisDatabase,
		})

		// test redis connection
		redisCheck, err := rdb.Ping(ctx).Result()
		if err != nil {
			panic(err)
		}

		// print redis response
		fmt.Println("Redis Local Ping: ", redisCheck)
	case "ring":
		// create map to hold redis nodes
		nodes := make(map[string]string, 0)
		// iterate over redis nodes formatting into the node map
		for i, n := range cfg.RedisConfig.RedisNodes {
			nodes[fmt.Sprintf("node%d", i)] = n
		}

		// create ring client
		rdb = redis.NewRing(&redis.RingOptions{
			Addrs:    nodes,
			Password: cfg.RedisConfig.RedisPassword,
			DB:       cfg.RedisConfig.RedisDatabase,
		})

		// test redis connection
		redisCheck, err := rdb.Ping(ctx).Result()
		if err != nil {
			panic(err)
		}

		// print redis response
		fmt.Println("Redis Ring Ping: ", redisCheck)
	case "cluster":
		// create cluster client
		rdb = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    cfg.RedisConfig.RedisNodes,
			Password: cfg.RedisConfig.RedisPassword,
		})

		// test redis connection
		redisCheck, err := rdb.ClusterNodes(ctx).Result()
		if err != nil {
			panic(err)
		}

		// print redis response
		fmt.Println("Redis Cluster Nodes:\n", redisCheck)
	default:
		panic(fmt.Sprintf("invalid option `%s` for `redisType`; options: local, ring, cluster", cfg.RedisConfig.RedisType))
	}

	// derive jetstream logger from root logger
	jetstreamLogger := rootLogger.WithName("gigo-core-jetstream")

	fmt.Println("Creating jetstream client")
	js, err := mq.NewJetstreamClient(cfg.JetstreamConfig, jetstreamLogger)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create jetstream client, %v", err))
	}

	fmt.Println("Creating gitea client")
	vcsClient, err := git.CreateVCSClient(cfg.GiteaConfig.HostUrl, cfg.GiteaConfig.Username, cfg.GiteaConfig.Password, false)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
	}

	// derive workspace provisioner logger from root logger
	wsProvisionerLogger := rootLogger.WithName("gigo-core-ws-provisioner")

	wsClientOpts := ws.WorkspaceClientOptions{
		Servers: make([]ws.WorkspaceClientConnectionOptions, 0),
		Logger:  wsProvisionerLogger,
	}

	for _, s := range cfg.WsConfig {
		wsClientOpts.Servers = append(wsClientOpts.Servers, ws.WorkspaceClientConnectionOptions{
			Host: s.Host,
			Port: s.Port,
		})
	}

	fmt.Println("Creating workspace client")
	wsClient, err := ws.NewWorkspaceClient(wsClientOpts)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create workspace client, %v", err))
	}

	// derive routine logger from  root logger
	routineLogger := rootLogger.WithName("gigo-core-routine")

	// derive cluster logger from root logger
	clusterLogger := rootLogger.WithName("gigo-core-cluster")

	streakLogger := rootLogger.WithName("gigo-core-streak")

	// create worker pool for follower routine using logical cores
	followerWorkerPool := pool.New()
	followerWorkerPool.WithMaxGoroutines(runtime.NumCPU())

	fmt.Println("Creating streak engine")
	// create streak engine
	streakEngine := streak.NewStreakEngine(tiDB, rdb, snowflakeNode, streakLogger)

	fmt.Println("Creating ws status updater")
	// create workspace status updater
	wsStatusUpdater := utils.NewWorkspaceStatusUpdater(utils.WorkspaceStatusUpdaterOptions{
		Js:       js,
		DB:       tiDB,
		Hostname: cfg.HTTPServerConfig.Hostname,
		Tls:      cfg.HTTPServerConfig.UseTLS,
	})

	fmt.Println("Creating ziti mesh")
	// create a new ziti mesh manager and server
	zitiManager, err := zitimesh.NewManager(cfg.ZitiConfig)
	if err != nil {
		log.Fatalf("failed to create ziti mesh manager: %v", err)
	}
	zitiServerID, zitiServerToken, err := zitiManager.CreateServer(nodeID)
	if err != nil {
		log.Fatalf("failed to create ziti server: %v", err)
	}
	zitiServer, err := zitimesh.NewServer(ctx, zitiServerID, zitiServerToken, systemLogger)
	if err != nil {
		log.Fatalf("failed to create ziti server: %v", err)
	}

	fmt.Println("Creating cluster node")
	// create cluster node
	var clusterNode cluster.Node
	if !cfg.Cluster {
		clusterNode = cluster.NewStandaloneNode(
			ctx,
			nodeID,
			// we assume that the node ip will always be set at this
			// env var - this is really designed to be operated on k8s
			// but could theoretically be set manually if deployed by hand
			os.Getenv("GIGO_POD_IP"),
			leader.Routine(nodeID, cfg, tiDB, js, rdb, wsStatusUpdater, routineLogger),
			follower.Routine(nodeID, cfg, tiDB, wsClient, vcsClient, js, followerWorkerPool, streakEngine, snowflakeNode, wsStatusUpdater, rdb, storageEngine, zitiManager, routineLogger),
			// we use a 1s tick for the cluster routines
			time.Second,
			clusterLogger,
		)
	} else {
		clusterNode, err = cluster.NewClusterNode(cluster.ClusterNodeOptions{
			Ctx: ctx,
			ID:  nodeID,
			// we assume that the node ip will always be set at this
			// env var - this is really designed to be operated on k8s
			// but could theoretically be set manually if deployed by hand
			Address: os.Getenv("GIGO_POD_IP"),
			// we have a 10 second timeout on the node such that if the node
			// dies or hangs for more than 10 seconds we will consider the node
			// dead and will elect a new leader forcing the node to exit the
			// cluster and rejoin with a new role
			Ttl:         time.Second * 10,
			ClusterName: "gigo-core",
			EtcdConfig: etcd.Config{
				Endpoints: cfg.EtcdConfig.Hosts,
				Username:  cfg.EtcdConfig.Username,
				Password:  cfg.EtcdConfig.Password,
			},
			LeaderRoutine:   leader.Routine(nodeID, cfg, tiDB, js, rdb, wsStatusUpdater, routineLogger),
			FollowerRoutine: follower.Routine(nodeID, cfg, tiDB, wsClient, vcsClient, js, followerWorkerPool, streakEngine, snowflakeNode, wsStatusUpdater, rdb, storageEngine, zitiManager, routineLogger),
			// we use a 1s tick for the cluster routines
			RoutineTick: time.Second,
			Logger:      clusterLogger,
		})
		if err != nil {
			log.Fatalf("failed to create cluster node: %v", err)
		}
	}

	fmt.Println("Starting cluster node")
	// start the cluster node
	clusterNode.Start()

	fmt.Println("Creating creating password filter")
	// create password filter
	passwordFilter, err := utils.NewPasswordFilter(storageEngine)
	if err != nil {
		log.Fatalf("failed to create password filter: %v", err)
	}

	whitelistedIpRanges := make([]*net.IPNet, 0)
	for _, ipnet := range cfg.HTTPServerConfig.WhitelistedIpRanges {
		_, ipnet, err := net.ParseCIDR(ipnet)
		if err != nil {
			log.Fatalf("failed to parse whitelisted ip range: %v", err)
		}
		whitelistedIpRanges = append(whitelistedIpRanges, ipnet)
	}

	fmt.Printf("Creating HTTP server @ %s:%s\n", cfg.HTTPServerConfig.Address, cfg.HTTPServerConfig.Port)
	// create HTTP server
	externalServer, err := external_api.CreateHTTPServer(cfg.HTTPServerConfig, cfg.OTELConfig.ServiceName, tiDB, meili, rdb, snowflakeNode,
		vcsClient, storageEngine, wsClient, js, wsStatusUpdater, parsedAccessUrl, passwordFilter, cfg.GithubSecret,
		cfg.HTTPServerConfig.ForceCdnAccess, cfg.HTTPServerConfig.CdnAccessKey, cfg.MasterKey, cfg.CaptchaSecret, whitelistedIpRanges, httpLogger)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create http server, %v", err))
	}

	fmt.Println("Creating workspace api server")
	// create workspace api server
	wsApiOpts := &api.WorkspaceAPIOptions{
		ID:          nodeID,
		ClusterNode: clusterNode,
		Ctx:         ctx,
		DerpMeshKey: cfg.DerpMeshKey,
		// we assume that the node ip will always be set at this
		// env var - this is really designed to be operated on k8s
		// but could theoretically be set manually if deployed by hand
		Address:        os.Getenv("GIGO_POD_IP"),
		Logger:         httpLogger,
		DB:             tiDB,
		StreakEngine:   streakEngine,
		RDB:            rdb,
		VcsClient:      vcsClient,
		SnowflakeNode:  snowflakeNode,
		AccessURL:      parsedAccessUrl,
		AppHostname:    cfg.HTTPServerConfig.Hostname,
		GitUseTLS:      strings.Contains(cfg.GiteaConfig.HostUrl, "https://"),
		Js:             js,
		RegistryCaches: cfg.RegistryCaches,
		ZitiServer:     zitiServer,
	}
	wsApiServer, err := api.NewWorkspaceAPI(wsApiOpts)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create workspace api server, %v", err))
	}

	fmt.Println("Linking workspace api to main http server")
	// link workspace api to main http server
	externalServer.LinkWorkspaceAPI(wsApiServer)

	// create channel to handle external server errors
	externalServerError := make(chan error)

	fmt.Println("Creating signal handlers")
	// register shutdown handler for all potential interrupt signals
	interrupt := tebata.New(syscall.SIGINT)
	err = interrupt.Reserve(shutdown, externalServer, zitiManager, clusterNode, followerWorkerPool, cancel, systemLogger)
	if err != nil {
		log.Fatal("failed to created interrupt handler: ", err)
	}

	term := tebata.New(syscall.SIGTERM)
	err = term.Reserve(shutdown, externalServer, zitiManager, clusterNode, followerWorkerPool, cancel, systemLogger)
	if err != nil {
		log.Fatal("failed to created term handler: ", err)
	}

	// this doesn't really work since sigkill overrides the handler but we do it anyway
	kill := tebata.New(syscall.SIGKILL)
	err = kill.Reserve(shutdown, externalServer, zitiManager, clusterNode, followerWorkerPool, cancel, systemLogger)
	if err != nil {
		log.Fatal("failed to created kill handler: ", err)
	}

	fmt.Println("Launching external API...")
	go func(err chan error) { err <- externalServer.Serve() }(externalServerError)

	// write to console informing of successful launch
	fmt.Println("Running...")

	// wait for external server to exit
	err, ok := <-externalServerError

	// acquire lock so we can be sure that any graceful shutdown has completed
	lock.Lock()
	defer lock.Unlock()

	// only log the error if we didn't gracefully shutdown
	if err == nil || interrupted {
		return
	}

	// handle unexpected channel closure
	if !ok {
		log.Fatal("unexpected channel closure in external server error channel")
	}

	// panic error
	log.Fatal(fmt.Errorf("external api server failure: %w", err))
}
