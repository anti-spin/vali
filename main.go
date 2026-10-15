package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

var (
	rdb           *redis.Client
	mariaDB       *sql.DB
	useRedis      bool
	useMariaDB    bool
	staticDomains []string
)

func main() {
	// Load static domains from JSON file if provided
	configFile := os.Getenv("VALI_DOMAINS_FILE")
	if configFile == "" {
		configFile = "vali_domains.json"
	}
	loadStaticDomains(configFile)

	// Load additional static domains directly from env (comma-separated: "example.com,*.example.net")
	if envDomains := os.Getenv("VALI_DOMAINS"); envDomains != "" {
		for _, d := range strings.Split(envDomains, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				staticDomains = append(staticDomains, d)
			}
		}
	}

	// Redis setup (enabled by default unless explicitly disabled)
	useRedis = os.Getenv("VALI_USE_REDIS") != "false"
	if useRedis {
		redisURL := os.Getenv("REDIS_URL")
		if redisURL == "" {
			redisURL = "localhost:6379"
		}

		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			opt = &redis.Options{Addr: redisURL}
		}

		rdb = redis.NewClient(opt)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Fatalf("Could not connect to Redis: %v", err)
		}
		log.Println("Connected to Redis")
	} else {
		log.Println("Redis integration disabled (VALI_USE_REDIS=false)")
	}

	// MariaDB / MySQL setup
	useMariaDB = os.Getenv("VALI_USE_MARIADB") == "true"
	if useMariaDB {
		mariaDBUser := os.Getenv("DB_USERNAME")
		mariaDBPass := os.Getenv("DB_PASSWORD")
		mariaDBHost := os.Getenv("DB_HOST")
		mariaDBPort := os.Getenv("DB_PORT")
		if mariaDBPort == "" {
			mariaDBPort = "3306"
		}
		mariaDBName := os.Getenv("DB_DATABASE")

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", mariaDBUser, mariaDBPass, mariaDBHost, mariaDBPort, mariaDBName)
		var err error
		mariaDB, err = sql.Open("mysql", dsn)
		if err != nil {
			log.Fatalf("Could not configure MariaDB pool: %v", err)
		}
		mariaDB.SetMaxOpenConns(25)
		mariaDB.SetMaxIdleConns(5)
		mariaDB.SetConnMaxLifetime(5 * time.Minute)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := mariaDB.PingContext(ctx); err != nil {
			log.Fatalf("Could not connect to MariaDB: %v", err)
		}
		log.Println("Connected to MariaDB")
	}

	http.HandleFunc("/healthz", healthCheckHandler)
	http.HandleFunc("/vali/", validateDomain)

	port := os.Getenv("VALI_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting vali service on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func loadStaticDomains(configFile string) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Printf("Notice: No static domain config file read (%s): %v", configFile, err)
		return
	}

	if err := json.Unmarshal(data, &staticDomains); err != nil {
		log.Printf("Warning: Could not parse static domains file: %v", err)
	}
}

func validateDomain(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		domain = strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/vali/"))
		if domain == "" {
			http.Error(w, "Domain parameter or path required", http.StatusBadRequest)
			return
		}
	}

	ctx := r.Context()

	// 1. Static Domain Match
	for _, staticDomain := range staticDomains {
		if strings.EqualFold(domain, staticDomain) ||
			(strings.HasPrefix(staticDomain, "*") && strings.HasSuffix(domain, strings.TrimPrefix(staticDomain, "*"))) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Domain is valid"))
			return
		}
	}

	// 2. Redis Lookup (if enabled)
	if useRedis && rdb != nil {
		exists, err := rdb.Exists(ctx, domain).Result()
		if err != nil {
			log.Printf("Error checking domain in Redis: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if exists == 1 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Domain is valid"))
			return
		}
	}

	// 3. MariaDB Lookup (if enabled)
	if useMariaDB && mariaDB != nil {
		query := "SELECT EXISTS (SELECT 1 FROM domains WHERE domain_name = ?)"
		var dbExists bool
		err := mariaDB.QueryRowContext(ctx, query, domain).Scan(&dbExists)
		if err != nil {
			log.Printf("Error checking domain in MariaDB: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if dbExists {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Domain is valid"))
			return
		}
	}

	http.Error(w, "Domain not found", http.StatusNotFound)
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if useRedis && rdb != nil {
		if err := rdb.Ping(ctx).Err(); err != nil {
			http.Error(w, "Redis unhealthy", http.StatusServiceUnavailable)
			return
		}
	}

	if useMariaDB && mariaDB != nil {
		if err := mariaDB.PingContext(ctx); err != nil {
			http.Error(w, "MariaDB unhealthy", http.StatusServiceUnavailable)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
