package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/readeck/readeck/configs"
)

func setHost(r *http.Request) error {
	xfh := r.Header.Get("X-Forwarded-Host")
	if xfh == "" {
		return nil
	}
	pair := strings.SplitN(xfh, ":", 2)
	host := pair[0]

	if len(pair) > 1 {
		port, err := strconv.ParseUint(pair[1], 10, 32)
		if err != nil {
			return err
		}

		r.Host = fmt.Sprintf("%s:%d", host, port)

	} else {
		r.Host = host
	}

	return nil
}

func checkHost(r *http.Request) error {
	host := r.Host
	port := r.URL.Port()
	if port != "" {
		host = strings.TrimSuffix(host, ":"+port)
	}
	host = strings.TrimSuffix(host, ".")

	for _, x := range configs.Config.Server.AllowedHosts {
		if x == host {
			return nil
		}
	}
	return fmt.Errorf("host is not allowed: %s", host)
}

func setProto(r *http.Request) error {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		return nil
	}
	if proto != "http" && proto != "https" {
		return fmt.Errorf("invalid x-forwarded-proto %s", proto)
	}
	r.URL.Scheme = proto
	return nil
}

// InitRequest update the scheme and host on the incoming
// HTTP request URL (r.URL), based on provided headers and/or
// current environnement.
//
// It also checks the validity of the host header when the server
// is not running in dev mode.
func (s *Server) InitRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set host
		if configs.Config.Server.UseXForwardedHost {
			if err := setHost(r); err != nil {
				s.Log(r).WithError(err).Error("server error")
				s.Status(w, r, http.StatusBadRequest)
				return
			}
		}
		r.URL.Host = r.Host

		// Check host
		if !configs.Config.Main.DevMode {
			if err := checkHost(r); err != nil {
				s.Log(r).WithError(err).Error("server error")
				s.Status(w, r, http.StatusBadRequest)
				return
			}
		}

		// Set scheme
		r.URL.Scheme = "http"
		if configs.Config.Server.UseXForwardedProto {
			if err := setProto(r); err != nil {
				s.Log(r).WithError(err).Error("server error")
				s.Status(w, r, http.StatusBadRequest)
				return
			}
		} else if r.TLS != nil {
			r.URL.Scheme = "https"
		}

		next.ServeHTTP(w, r)
	})
}

// SetSecurityHeaders adds some headers to improve client side security.
func (s *Server) SetSecurityHeaders() func(next http.Handler) http.Handler {
	cspHeader := strings.Join([]string{
		"default-src 'self'",
		"img-src 'self' data:",
		"media-src 'self' data:",
		"style-src 'self' 'unsafe-inline'",
		"child-src *", // Allow iframes for videos
	}, "; ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Referrer-Policy", "same-origin")
			w.Header().Add("Content-Security-Policy", cspHeader)
			w.Header().Add("X-Frame-Options", "DENY")
			w.Header().Add("X-Content-Type-Options", "nosniff")
			w.Header().Add("X-XSS-Protection", "1; mode=block")

			next.ServeHTTP(w, r)
		})
	}
}
