package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"
)

// LogEntry represents structured log standard for Elasticsearch & Kibana
type LogEntry struct {
	Timestamp  string                 `json:"@timestamp"`
	Service    string                 `json:"service"`
	TraceID    string                 `json:"trace_id"`
	Level      string                 `json:"level"` // INFO, WARN, ERROR
	Message    string                 `json:"message"`
	HTTPMethod string                 `json:"http_method,omitempty"`
	HTTPPath   string                 `json:"http_path,omitempty"`
	HTTPStatus int                    `json:"http_status,omitempty"`
	DurationMs int64                  `json:"duration_ms,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
	Extra      map[string]interface{} `json:"extra,omitempty"`
}

type AppConfig struct {
	Port         string
	ServiceName  string
	ElasticURL   string // if set, ship directly to Elasticsearch index
	IndexName    string
	ElasticUser  string
	ElasticPass  string
}

type Server struct {
	cfg        AppConfig
	httpClient *http.Client
	// Simulated DB connection pool
	dbPoolMu sync.Mutex
	dbActive int
	dbLimit  int
}

func main() {
	cfg := AppConfig{
		Port:        getEnv("PORT", "8000"),
		ServiceName: getEnv("SERVICE_NAME", "order-service"),
		ElasticURL:  getEnv("ELASTICSEARCH_URL", ""),
		IndexName:   getEnv("LOGS_INDEX", "logs-app-dev"),
		ElasticUser: getEnv("ELASTICSEARCH_USERNAME", "elastic"),
		ElasticPass: getEnv("ELASTICSEARCH_PASSWORD", "changeme"),
	}

	srv := &Server{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		dbActive: 0,
		dbLimit:  5, // max 5 simulated connections
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", srv.handleHealth)
	mux.HandleFunc("/api/orders", srv.handleOrders)

	// Simulated Failure Scenarios
	mux.HandleFunc("/api/debug/db-pool-exhaustion", srv.handleDBPoolExhaustion)
	mux.HandleFunc("/api/debug/external-timeout", srv.handleExternalTimeout)
	mux.HandleFunc("/api/debug/panic-nil-pointer", srv.handlePanicNilPointer)

	// Apply structured logging & recovery middleware
	handler := srv.withLoggingMiddleware(mux)

	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	go func() {
		log.Printf("[%s] Server running on port %s", cfg.ServiceName, cfg.Port)
		if cfg.ElasticURL != "" {
			log.Printf("[%s] Log shipping enabled -> %s (Index: %s)", cfg.ServiceName, cfg.ElasticURL, cfg.IndexName)
		} else {
			log.Printf("[%s] Log shipping: stdout only (ELASTICSEARCH_URL not set)", cfg.ServiceName)
		}
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	log.Println("Server exiting")
}

// Middleware: Structured access logging & panic recovery
func (s *Server) withLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = generateTraceID()
		}
		w.Header().Set("X-Trace-ID", traceID)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			duration := time.Since(start).Milliseconds()
			if recErr := recover(); recErr != nil {
				rec.status = http.StatusInternalServerError
				stack := string(debug.Stack())
				s.emitLog(LogEntry{
					Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
					Service:    s.cfg.ServiceName,
					TraceID:    traceID,
					Level:      "ERROR",
					Message:    fmt.Sprintf("Panic recovered: %v", recErr),
					HTTPMethod: r.Method,
					HTTPPath:   r.URL.Path,
					HTTPStatus: http.StatusInternalServerError,
					DurationMs: duration,
					StackTrace: stack,
				})
				http.Error(w, `{"error":"Internal server panic"}`, http.StatusInternalServerError)
				return
			}

			// Don't duplicate if handlers already emitted specific ERROR logs
			if rec.status < http.StatusBadRequest {
				s.emitLog(LogEntry{
					Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
					Service:    s.cfg.ServiceName,
					TraceID:    traceID,
					Level:      "INFO",
					Message:    fmt.Sprintf("Request processed %s %s", r.Method, r.URL.Path),
					HTTPMethod: r.Method,
					HTTPPath:   r.URL.Path,
					HTTPStatus: rec.status,
					DurationMs: duration,
				})
			}
		}()

		// Inject traceID into context
		ctx := context.WithValue(r.Context(), traceIDKey, traceID)
		next.ServeHTTP(rec, r.WithContext(ctx))
	})
}

// 1. Health check
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"UP","timestamp":"` + time.Now().UTC().Format(time.RFC3339) + `"}`))
}

// 2. Normal Order endpoint
func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	traceID := getTraceID(r.Context())
	w.Header().Set("Content-Type", "application/json")

	// Simulate light work
	time.Sleep(time.Duration(20+rand.Intn(40)) * time.Millisecond)

	s.emitLog(LogEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Service:    s.cfg.ServiceName,
		TraceID:    traceID,
		Level:      "INFO",
		Message:    "Order created successfully for customer_id=cust_8921",
		HTTPMethod: r.Method,
		HTTPPath:   r.URL.Path,
		HTTPStatus: http.StatusCreated,
		Extra: map[string]interface{}{
			"order_id": fmt.Sprintf("ord_%d", time.Now().UnixNano()),
			"amount":   1290.50,
			"items":    2,
		},
	})

	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"status":"success","message":"Order created successfully"}`))
}

// 3. Scenario A: DB Connection Pool Exhaustion (Deadlock / Leak)
func (s *Server) handleDBPoolExhaustion(w http.ResponseWriter, r *http.Request) {
	traceID := getTraceID(r.Context())
	w.Header().Set("Content-Type", "application/json")

	s.dbPoolMu.Lock()
	s.dbActive++
	current := s.dbActive
	s.dbPoolMu.Unlock()

	// Simulate connection checkout timeout if active exceeds limit
	if current > s.dbLimit {
		s.emitLog(LogEntry{
			Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
			Service:    s.cfg.ServiceName,
			TraceID:    traceID,
			Level:      "ERROR",
			Message:    fmt.Sprintf("HikariPool-1 - Connection is not available, request timed out after 30000ms (active=%d, max=%d)", current, s.dbLimit),
			HTTPMethod: r.Method,
			HTTPPath:   r.URL.Path,
			HTTPStatus: http.StatusInternalServerError,
			StackTrace: "org.postgresql.util.PSQLException: Connection to 10.0.4.12:5432 refused\n\tat com.zaxxer.hikari.pool.HikariPool.getConnection(HikariPool.java:213)\n\tat com.company.order.repository.OrderRepository.acquireConnection(OrderRepository.go:88)\n\tat com.company.order.service.CheckoutService.processCheckout(CheckoutService.go:142)",
			Extra: map[string]interface{}{
				"db_active_connections": current,
				"db_max_connections":    s.dbLimit,
				"error_code":             "DB_POOL_EXHAUSTED",
			},
		})

		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"Database connection pool exhausted. Unable to process transaction."}`))
		return
	}

	// Simulating leaked connection not returned to pool
	s.emitLog(LogEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Service:    s.cfg.ServiceName,
		TraceID:    traceID,
		Level:      "WARN",
		Message:    fmt.Sprintf("DB connection acquired without active release hook (active_connections=%d/%d)", current, s.dbLimit),
		HTTPMethod: r.Method,
		HTTPPath:   r.URL.Path,
		HTTPStatus: http.StatusOK,
	})

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"connection_held","active_connections":%d,"limit":%d,"hint":"Call multiple times to trigger pool exhaustion"}`, current, s.dbLimit)))
}

// 4. Scenario B: 3rd-party Payment Gateway Timeout (HTTP 504 Gateway Timeout)
func (s *Server) handleExternalTimeout(w http.ResponseWriter, r *http.Request) {
	traceID := getTraceID(r.Context())
	w.Header().Set("Content-Type", "application/json")

	// Warning log before timeout
	s.emitLog(LogEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Service:    s.cfg.ServiceName,
		TraceID:    traceID,
		Level:      "WARN",
		Message:    "Slow response detected from upstream payment provider (payment-gateway.partner.com)",
		HTTPMethod: r.Method,
		HTTPPath:   r.URL.Path,
		Extra: map[string]interface{}{
			"upstream_url": "https://payment-gateway.partner.com/v2/charges",
			"timeout_ms":   5000,
		},
	})

	// Error log after timeout
	s.emitLog(LogEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Service:    s.cfg.ServiceName,
		TraceID:    traceID,
		Level:      "ERROR",
		Message:    "Upstream payment gateway timeout: context deadline exceeded after 5000ms",
		HTTPMethod: r.Method,
		HTTPPath:   r.URL.Path,
		HTTPStatus: http.StatusGatewayTimeout,
		StackTrace: "net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)\n\tat payment_client.go:76\n\tat checkout_handler.go:210",
		Extra: map[string]interface{}{
			"upstream_url": "https://payment-gateway.partner.com/v2/charges",
			"error_code":   "UPSTREAM_TIMEOUT",
		},
	})

	w.WriteHeader(http.StatusGatewayTimeout)
	_, _ = w.Write([]byte(`{"error":"Payment provider did not respond in time (504 Gateway Timeout)"}`))
}

// 5. Scenario C: Nil Pointer Dereference (Panic & Crash)
func (s *Server) handlePanicNilPointer(w http.ResponseWriter, r *http.Request) {
	type CustomerProfile struct {
		Name    string
		Address *string
	}
	var profile *CustomerProfile // intentional nil pointer

	// This triggers panic and gets caught by middleware
	_ = strings.ToUpper(profile.Name)
}

// emitLog outputs structured JSON to STDOUT and optionally ships to Elasticsearch
func (s *Server) emitLog(entry LogEntry) {
	payload, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Failed to marshal log: %v", err)
		return
	}

	// Always print to stdout
	fmt.Println(string(payload))

	// Optionally ship directly to Elasticsearch if configured
	if s.cfg.ElasticURL != "" {
		go func(data []byte) {
			url := fmt.Sprintf("%s/%s/_doc", strings.TrimRight(s.cfg.ElasticURL, "/"), s.cfg.IndexName)
			req, err := http.NewRequest("POST", url, bytes.NewReader(data))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			if s.cfg.ElasticUser != "" {
				req.SetBasicAuth(s.cfg.ElasticUser, s.cfg.ElasticPass)
			}
			resp, err := s.httpClient.Do(req)
			if err == nil && resp != nil {
				_ = resp.Body.Close()
			}
		}(payload)
	}
}

// Helper utilities
type contextKey string

const traceIDKey contextKey = "trace_id"

func getTraceID(ctx context.Context) string {
	if val, ok := ctx.Value(traceIDKey).(string); ok && val != "" {
		return val
	}
	return generateTraceID()
}

func generateTraceID() string {
	return fmt.Sprintf("tr-%x-%d", rand.Int63n(1e9), time.Now().UnixNano()%100000)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
