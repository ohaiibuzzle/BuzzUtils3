package getimages

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

// fakeZerochan serves canned Zerochan responses and points both clients at it.
func fakeZerochan(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfg, []byte("user_agent: test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.LoadConfig(cfg); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/Eula":
			// Like the real site: the alias redirect drops the query string.
			http.Redirect(w, r, "/Eula+Lawrence", http.StatusMovedPermanently)
		case "/Eula+Lawrence":
			if !r.URL.Query().Has("json") || r.URL.Query().Get("l") != "200" {
				w.Write([]byte("<html>not json</html>"))
				return
			}
			w.Write([]byte(`{"items":[{"id":1,"tags":["Eula Lawrence"]},{"id":2,"tags":["nipples"]}]}`))
		case "/1":
			w.Write([]byte(`{"id":1,"large":"https://s1.zerochan.net/1.jpg","primary":"Eula Lawrence","tags":["Eula Lawrence","Genshin Impact"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	target, _ := url.Parse(srv.URL)
	oldZ, oldH := zerochanClient.Transport, httpClient.Transport
	rewrite := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme, req.URL.Host = target.Scheme, target.Host
		return http.DefaultTransport.RoundTrip(req)
	})
	zerochanClient.Transport, httpClient.Transport = rewrite, rewrite
	t.Cleanup(func() { zerochanClient.Transport, httpClient.Transport = oldZ, oldH })
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestZerochanResults(t *testing.T) {
	fakeZerochan(t)

	results, err := getZerochanResults("Eula")
	if err != nil {
		t.Fatalf("alias redirect: %v", err)
	}
	if len(results) != 2 || results[0].ID != 1 {
		t.Fatalf("alias redirect: got %+v", results)
	}

	results, err = getZerochanResults("Nobody")
	if err != nil || results != nil {
		t.Errorf("unknown tag: got %+v, %v, want nothing", results, err)
	}

	detail, err := getZerochanDetail(1)
	if err != nil {
		t.Fatal(err)
	}
	embed := makeZerochanEmbed(detail)
	if embed.Title != "Eula Lawrence" || embed.URL != "https://www.zerochan.net/1" || embed.Image.URL != detail.Large {
		t.Errorf("embed = %+v", embed)
	}

	if _, err := getZerochanDetail(404); err == nil {
		t.Error("missing post didn't error")
	}
}

func TestHasBannedTag(t *testing.T) {
	if !hasBannedTag([]string{"Eula Lawrence", "nipples"}) {
		t.Error("banned tag check is case sensitive")
	}
	if hasBannedTag([]string{"Eula Lawrence"}) {
		t.Error("clean tags flagged as banned")
	}
}
