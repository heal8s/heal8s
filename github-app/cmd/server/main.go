package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"

	"github.com/heal8s/heal8s/github-app/internal/config"
	"github.com/heal8s/heal8s/github-app/internal/github"
	"github.com/heal8s/heal8s/github-app/internal/k8s"
	"github.com/heal8s/heal8s/github-app/internal/remediation"
)

func main() {
	var configPath string
	var useEnv bool

	flag.StringVar(&configPath, "config", "config/config.yaml", "Path to configuration file")
	flag.BoolVar(&useEnv, "env", false, "Load configuration from environment variables")
	flag.Parse()

	// Setup logger
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	logger := zapr.NewLogger(zapLog)

	logger.Info("starting heal8s GitHub App service")

	// Load configuration
	var cfg *config.Config
	if useEnv {
		cfg, err = config.LoadFromEnv()
		if err != nil {
			logger.Error(err, "failed to load configuration from environment")
			os.Exit(1)
		}
		logger.Info("loaded configuration from environment")
	} else {
		cfg, err = config.LoadConfig(configPath)
		if err != nil {
			logger.Error(err, "failed to load configuration", "path", configPath)
			os.Exit(1)
		}
		logger.Info("loaded configuration from file", "path", configPath)
	}

	// Create Kubernetes client
	logger.Info("creating Kubernetes client", "kubeconfig", cfg.Kubernetes.Kubeconfig)
	k8sClient, err := k8s.NewClient(cfg.Kubernetes.Kubeconfig)
	if err != nil {
		logger.Error(err, "failed to create Kubernetes client")
		os.Exit(1)
	}
	logger.Info("Kubernetes client created successfully")

	// Create GitHub client
	logger.Info("creating GitHub client")
	githubClient, err := github.NewClient(
		cfg.GitHub.AppID,
		cfg.GitHub.InstallationID,
		cfg.GitHub.PrivateKeyPath,
	)
	if err != nil {
		logger.Error(err, "failed to create GitHub client")
		os.Exit(1)
	}
	logger.Info("GitHub client created successfully")

	// Parse poll interval
	pollInterval, err := time.ParseDuration(cfg.Processor.PollInterval)
	if err != nil {
		logger.Error(err, "failed to parse poll interval", "value", cfg.Processor.PollInterval)
		os.Exit(1)
	}

	// Create processor
	processor := remediation.NewProcessor(
		k8sClient,
		githubClient,
		logger.WithName("processor"),
		cfg.Kubernetes.Namespace,
	)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start processor in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := processor.Run(ctx, pollInterval); err != nil && err != context.Canceled {
			errChan <- err
		}
	}()

	logger.Info("heal8s GitHub App service started", "pollInterval", pollInterval)

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()
	case err := <-errChan:
		logger.Error(err, "processor error")
		cancel()
		os.Exit(1)
	}

	// Give processor time to shut down gracefully
	time.Sleep(2 * time.Second)
	logger.Info("heal8s GitHub App service stopped")
}
