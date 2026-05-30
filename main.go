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
	deleteAccountTmpl *template.Template
)

// initTemplates initializes the page-specific template pools to avoid block namespace collision.
func initTemplates() {
	layoutPath := filepath.Join("templates", "layout.html")

	indexTmpl = template.Must(template.ParseFiles(layoutPath, filepath.Join("templates", "index.html")))
	bmpCpTransactTmpl = template.Must(template.ParseFiles(layoutPath, filepath.Join("templates", "bmp_cp_transact.html")))
	deleteAccountTmpl = template.Must(template.ParseFiles(layoutPath, filepath.Join("templates", "delete_account.html")))
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

	mux.HandleFunc("/apps/bmp-cp-transact/delete-account", func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			PageData
			Success      bool
			ErrorMessage string
			CPCode       string
			Phone        string
			FullName     string
		}{
			PageData: getPageData("Delete Account Request - BMP CP Transact"),
		}

		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				data.ErrorMessage = "Failed to process request. Please try again."
				renderPage(w, deleteAccountTmpl, data)
				return
			}

			data.CPCode = r.FormValue("cp_code")
			data.Phone = r.FormValue("phone")
			data.FullName = r.FormValue("full_name")
			confirmDelete := r.FormValue("confirm_delete")
			reason := r.FormValue("reason")

			if data.CPCode == "" || data.Phone == "" || data.FullName == "" {
				data.ErrorMessage = "Please fill in all required fields (Full Name, CP Code, and Phone Number)."
				renderPage(w, deleteAccountTmpl, data)
				return
			}

			if confirmDelete != "yes" {
				data.ErrorMessage = "You must check the box to confirm you want to delete your account."
				renderPage(w, deleteAccountTmpl, data)
				return
			}

			// Audit log to Cloud Logging stdout
			log.Printf("[AUDIT_LOG][DELETION_REQUEST] App: BMP CP Transact | Name: %s | CP Code: %s | Phone: %s | Reason: %s | Time: %s",
				data.FullName, data.CPCode, data.Phone, reason, time.Now().Format(time.RFC3339))

			data.Success = true
		}

		renderPage(w, deleteAccountTmpl, data)
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

	log.Printf("Starting secure server on port %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server startup failed: %v", err)
	}
}
