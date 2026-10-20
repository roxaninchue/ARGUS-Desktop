package cloud

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAvatarUpload(t *testing.T) {
	var gotType, gotAuth string
	var gotLen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/me/avatar":
			b, _ := io.ReadAll(r.Body)
			gotType, gotAuth, gotLen = r.Header.Get("Content-Type"), r.Header.Get("Authorization"), len(b)
			w.Write([]byte(`{"ok":true,"avatar_url":"https://x/v1/avatar/u1?v=2","user":{"id":"u1","avatar_url":"https://x/v1/avatar/u1?v=2"}}`))
		case "/v1/me/avatar/delete":
			w.Write([]byte(`{"ok":true,"avatar_url":"","user":{"id":"u1","avatar_url":""}}`))
		}
	}))
	defer srv.Close()
	c := &Client{Base: srv.URL, HC: srv.Client()}
	u, err := c.AvatarUpload(context.Background(), "tok", "image/webp", []byte("RIFF....WEBP"))
	if err != nil || u.AvatarURL == "" || gotType != "image/webp" || gotAuth != "Bearer tok" || gotLen != 12 {
		t.Fatalf("upload: %v %+v %q %q %d", err, u, gotType, gotAuth, gotLen)
	}
	if _, err := c.AvatarUpload(context.Background(), "tok", "image/gif", []byte("x")); err == nil {
		t.Fatal("gif must be refused before sending")
	}
	if _, err := c.AvatarUpload(context.Background(), "tok", "image/png", make([]byte, MaxAvatar+1)); err == nil {
		t.Fatal("over 256 KB must be refused before sending")
	}
	u, err = c.AvatarDelete(context.Background(), "tok")
	if err != nil || u.AvatarURL != "" {
		t.Fatalf("delete: %v %+v", err, u)
	}
}
