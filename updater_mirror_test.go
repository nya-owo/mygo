package mygo

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// An update on GitHub, which some networks cannot reach, comes through a
// mirror instead, as the mirror's prefix and the GitHub URL.
func TestUpdateFromMirror(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, r.URL.Path)
	}))
	defer srv.Close()
	tr := srv.Client().Transport.(*http.Transport).Clone()
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if strings.HasPrefix(addr, "github.com") {
			return nil, errors.New("blocked")
		}
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	old := updateClient
	defer func() { updateClient = old; Updater.SetMirrors() }()
	updateClient = &http.Client{Transport: tr}

	const feed = "https://github.com/me/app/releases/latest/download/update.json"
	if _, err := httpGet(context.Background(), feed); err == nil {
		t.Fatal("GitHub answered with no mirror set")
	}
	Updater.SetMirrors(srv.URL + "/")
	body, err := httpGet(context.Background(), feed)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	got, _ := io.ReadAll(body)
	if !strings.HasSuffix(string(got), "github.com/me/app/releases/latest/download/update.json") {
		t.Errorf("the mirror was asked for %q", got)
	}
}

// A listener who chose a mirror has GitHub left alone: the mirror is asked
// first, even when GitHub would answer.
func TestUpdateMirrorsOnly(t *testing.T) {
	var asked []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.Host+r.URL.Path)
		io.WriteString(w, "ok")
	}))
	defer srv.Close()
	tr := srv.Client().Transport.(*http.Transport).Clone()
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	old := updateClient
	defer func() { updateClient = old; Updater.SetMirrors(); Updater.SetMirrorsOnly(false) }()
	updateClient = &http.Client{Transport: tr}

	const feed = "https://github.com/me/app/releases/latest/download/update.json"
	Updater.SetMirrors("https://mirror.example.com/")
	Updater.SetMirrorsOnly(true)
	body, err := httpGet(context.Background(), feed)
	if err != nil {
		t.Fatal(err)
	}
	body.Close()
	if len(asked) != 1 || !strings.HasPrefix(asked[0], "mirror.example.com/") {
		t.Errorf("asked %q, want only the mirror", asked)
	}
}
