package sonar

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewTrimsTrailingSlash(t *testing.T) {
	if got := New("http://localhost:9000/").BaseURL; got != "http://localhost:9000" {
		t.Errorf("BaseURL = %q, want without trailing slash", got)
	}
}

func TestStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/system/status" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(`{"status":"UP"}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).Status()
	if err != nil {
		t.Fatal(err)
	}
	if got != "UP" {
		t.Errorf("Status = %q, want UP", got)
	}
}

func TestStatusUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // close immediately so the server is unreachable

	if _, err := New(url).Status(); err == nil {
		t.Error("expected error when server is unreachable")
	}
}

func TestCheckAdmin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if !c.CheckAdmin("secret") {
		t.Error("CheckAdmin(secret) = false, want true")
	}
	if c.CheckAdmin("wrong") {
		t.Error("CheckAdmin(wrong) = true, want false")
	}
}

func TestChangeAdminPassword(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("login") != "admin" ||
			r.FormValue("previousPassword") != "old" ||
			r.FormValue("password") != "new" {
			t.Errorf("unexpected form: %v", r.Form)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := New(srv.URL).ChangeAdminPassword("old", "new"); err != nil {
		t.Fatal(err)
	}
}

func TestChangeAdminPasswordError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "policy violation", http.StatusBadRequest)
	}))
	defer srv.Close()

	err := New(srv.URL).ChangeAdminPassword("old", "weak")
	if err == nil {
		t.Fatal("expected error on HTTP 400")
	}
}

func TestGenerateToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("name") != "ephemeral" || r.FormValue("type") != "GLOBAL_ANALYSIS_TOKEN" {
			t.Errorf("unexpected form: %v", r.Form)
		}
		w.Write([]byte(`{"token":"abc123"}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).GenerateToken("admin", "ephemeral")
	if err != nil {
		t.Fatal(err)
	}
	if got != "abc123" {
		t.Errorf("token = %q, want abc123", got)
	}
}

func TestGenerateTokenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"token":""}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL).GenerateToken("admin", "x"); err == nil {
		t.Error("expected error on empty token")
	}
}

func TestRevokeToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("name") != "ephemeral" {
			t.Errorf("unexpected name %q", r.FormValue("name"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := New(srv.URL).RevokeToken("admin", "ephemeral"); err != nil {
		t.Fatal(err)
	}
}

func TestRevokeTokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	if err := New(srv.URL).RevokeToken("admin", "missing"); err == nil {
		t.Error("expected error on HTTP 404")
	}
}
