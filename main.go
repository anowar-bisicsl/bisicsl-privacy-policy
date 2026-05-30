package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	indexTmpl         *template.Template
	bmpCpTransactTmpl *template.Template
)

// initTemplates initializes the page-specific template pools to avoid block namespace collision.
func initTemplates() {
	layoutPath := filepath.Join("templates", "layout.html")

	indexTmpl = template.Must(template.ParseFiles(layoutPath, filepath.Join("templates", "index.html")))
	bmpCpTransactTmpl = template.Must(template.ParseFiles(layoutPath, filepath.Join("templates", "bmp_cp_transact.html")))
}

// securityHeaders middleware injects production-grade HTTP security headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")
		// Prevent MIME-sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// XSS protection for older browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		// HTTP Strict Transport Security (HSTS)
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		// Referrer Policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Content Security Policy (CSP)
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; script-src 'self' 'unsafe-inline'; frame-ancestors 'none';")
		// Cache Control
		w.Header().Set("Cache-Control", "public, max-age=3600, must-revalidate")

		next.ServeHTTP(w, r)
	})
}

// renderPage executes the page's isolated layout.
func renderPage(w http.ResponseWriter, tmpl *template.Template, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// PageData holds standard dynamic values for the layout and body.
type PageData struct {
	Title       string
	CurrentYear int
}

func getPageData(title string) PageData {
	return PageData{
		Title:       title,
		CurrentYear: time.Now().Year(),
	}
}

func main() {
	initTemplates()

	mux := http.NewServeMux()

	// Static assets handler
	staticDir := filepath.Clean("static")
	fileServer := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// Routes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only match direct homepage path to avoid capturing everything
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		renderPage(w, indexTmpl, getPageData("BISICSL - App Privacy Policies"))
	})

	mux.HandleFunc("/apps/bmp-cp-transact", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, bmpCpTransactTmpl, getPageData("Privacy Policy - BMP CP Transact"))
	})

	// Wrap routing with security headers middleware
	handler := securityHeaders(mux)

	// Configure server with reasonable timeouts to prevent connection-exhaustion attacks
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Address normalization (e.g. for environment variables)
	if !strings.Contains(port, ":") {
		port = ":" + port
	}

	server := &http.Server{
		Addr:         port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Starting secure BISICSL Privacy Policy server on port %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server startup failed: %v", err)
	}
}
