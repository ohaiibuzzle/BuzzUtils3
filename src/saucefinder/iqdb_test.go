package saucefinder

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// Trimmed from a real iqdb.org response
const iqdbFixture = `<html><body><h2>Search results</h2>
<div id='pages' class='pages'><div><table><tr><th>Your image</th></tr><tr><td class='image'><img src='/thu/thu_a0666eb9.jpg' alt="[IMG]" width='150' height='84'></td></tr><tr><td><span title='img.jpg'>img.jpg</span></td></tr><tr><td>2311×1300</td></tr></table></div>
<div><table><tr><th>Best match</th></tr><tr><td class='image'><a href="//danbooru.donmai.us/posts/12228973"><img src='/danbooru/a/b/0/ab0b.jpg' alt="Rating: g Tags: hu_tao_(genshin_impact)" title="Rating: g"></a></td></tr><tr><td><img alt="icon" src="/icon/danbooru.ico" class="service-icon">Danbooru <span class="el"><a href="//gelbooru.com/index.php?page=post&s=list&md5=ab0b"><img alt="icon" src="/icon/gelbooru.png" class="service-icon">Gelbooru</a></span></td></tr><tr><td>2311×1300 [Unrated]</td></tr><tr><td>98% similarity</td></tr></table></div>
<br><div><table><tr><th>Additional match</th></tr><tr><td class='image'><a href="https://yande.re/post/show/1"><img src='/yandere/1/2/3.jpg' alt="Rating: e"></a></td></tr><tr><td>yande.re</td></tr><tr><td>800×600 [Explicit]</td></tr><tr><td>71% similarity</td></tr></table></div>
</div></body></html>`

func TestParseIqdbResults(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(iqdbFixture))
	if err != nil {
		t.Fatal(err)
	}
	matches := parseIqdbResults(doc)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d: %+v", len(matches), matches)
	}

	best := matches[0]
	if best.Link != "https://danbooru.donmai.us/posts/12228973" {
		t.Errorf("unexpected link %q", best.Link)
	}
	if best.Thumbnail != "https://iqdb.org/danbooru/a/b/0/ab0b.jpg" {
		t.Errorf("unexpected thumbnail %q", best.Thumbnail)
	}
	if best.Rating != "Unrated" || best.Similarity != "98%" {
		t.Errorf("unexpected rating/similarity %q / %q", best.Rating, best.Similarity)
	}
	if matches[1].Rating != "Explicit" || matches[1].Link != "https://yande.re/post/show/1" {
		t.Errorf("unexpected second match %+v", matches[1])
	}
}
