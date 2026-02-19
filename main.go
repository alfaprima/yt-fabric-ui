package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"fabric-agents/auth"
	"fabric-agents/core"
	"fabric-agents/web"
	"fabric-agents/yt"
	"log/slog"
)

func main() {
	// Initialize the logger
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Set the default logging level
	})
	logger := slog.New(logHandler)

	port := flag.String("port", "8085", "Port for the web server")
	flag.Parse()

	runWebServer(*port, logger)
}

func runWebServer(port string, logger *slog.Logger) {
	processor := core.NewProcessor(logger, "data/videos", yt.NewYT(""))
	store := auth.NewJSONStore("data/auth/store.json")
	authService := auth.NewService(store)
	core.SetGlobalFabricRunOptions(core.FabricRunOptions{
		Temperature:      getenvOptionalFloat64("FABRIC_GLOBAL_TEMPERATURE"),
		TopP:             getenvOptionalFloat64("FABRIC_GLOBAL_TOP_P"),
		PresencePenalty:  getenvOptionalFloat64("FABRIC_GLOBAL_PRESENCE_PENALTY"),
		FrequencyPenalty: getenvOptionalFloat64("FABRIC_GLOBAL_FREQUENCY_PENALTY"),
		Raw:              getenvBool("FABRIC_GLOBAL_RAW", false),
	})

	authEnabled := getenvBool("AUTH_ENABLED", false)
	if dur := os.Getenv("AUTH_SESSION_DURATION"); dur != "" {
		if parsed, err := time.ParseDuration(dur); err == nil {
			authService.SetSessionDuration(parsed)
		}
	}

	if err := authService.BootstrapAdmin(os.Getenv("AUTH_BOOTSTRAP_ADMIN_USER"), os.Getenv("AUTH_BOOTSTRAP_ADMIN_PASS")); err != nil {
		logger.Error("Failed to bootstrap admin", "error", err)
	}

	handler := web.NewHandler(processor, "data/videos", logger, authService, authEnabled, getenvBool("AUTH_COOKIE_SECURE", false))
	http.Handle("/", handler)
	logger.Info("Starting web server", "port", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}

func getenvBool(key string, defaultValue bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultValue
	}
	return b
}

func getenvOptionalFloat64(key string) *float64 {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return &f
}
