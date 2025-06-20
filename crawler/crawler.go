package crawler

import (
	"bytes"
	"context"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/calpa/urusai/config"
)

// Crawler generates random HTTP traffic starting from a set of roots.
// It respects depth and timeout limits, avoids already‑visited URLs and
// extracts links with the standard library HTML tokenizer for robustness.
// All network calls honour the supplied context so callers can cancel
// the crawl at any time (e.g. when a global deadline or signal fires).
//
// Public API is intentionally small — call New() then Crawl(ctx).
// The crawler retains no global state and can be created many times in
// one process or test.

type Crawler struct {
	cfg       *config.Config
	client    *http.Client
	rand      *rand.Rand
	startTime time.Time

	links   []string            // queue of links to visit next
	visited map[string]struct{} // fast membership test to avoid repeats
}

// New returns a ready‑to‑use Crawler. A fresh PRNG is seeded so that
// tests can supply their own *rand.Source when determinism is required.
func NewCrawler(cfg *config.Config) *Crawler {
	return &Crawler{
		cfg:     cfg,
		client:  &http.Client{Timeout: 5 * time.Second},
		rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
		visited: make(map[string]struct{}),
	}
}

// Crawl walks the Web until one of the following happens:
//   - The supplied context is cancelled
//   - Global timeout (cfg.Timeout) elapses
//   - Maximum link depth (cfg.MaxDepth) is reached
func (c *Crawler) Crawl(ctx context.Context) {
	c.startTime = time.Now()

	for {
		if ctx.Err() != nil || c.isTimeoutReached() {
			return
		}

		root := c.cfg.RootURLs[c.rand.Intn(len(c.cfg.RootURLs))]
		body, err := c.fetch(ctx, root)
		if err != nil {
			log.Printf("root fetch %s: %v", root, err)
			continue
		}

		c.links = c.extractLinks(body, root)
		if len(c.links) == 0 {
			continue
		}

		c.depthFirst(ctx, 0)
	}
}

// fetch performs a single HTTP GET, returns the page body (max 1 MiB).
func (c *Crawler) fetch(ctx context.Context, raw string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgents[c.rand.Intn(len(c.cfg.UserAgents))])

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	log.Printf("fetch %s: %s, Gorutine: %d", raw, resp.Status, runtime.NumGoroutine())
	defer resp.Body.Close()

	return io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB safety cap
}

// extractLinks returns all acceptable links found in the supplied HTML.
// It uses the html tokenizer instead of brittle regexes.
func (c *Crawler) extractLinks(body []byte, base string) []string {
	z := html.NewTokenizer(bytes.NewReader(body))
	baseURL, _ := url.Parse(base)

	var out []string
	for {
		switch z.Next() {
		case html.ErrorToken:
			return out
		case html.StartTagToken:
			t := z.Token()
			if t.DataAtom != atom.A {
				continue
			}
			for _, a := range t.Attr {
				if a.Key != "href" {
					continue
				}
				href := c.normalize(a.Val, baseURL)
				if c.accept(href) {
					out = append(out, href)
				}
			}
		}
	}
}

// normalize resolves relative links against base and tidies schemeless // URLs.
func (c *Crawler) normalize(href string, base *url.URL) string {
	if strings.HasPrefix(href, "//") {
		return base.Scheme + ":" + href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).String()
}

// accept applies validation, blacklist and dedup rules.
func (c *Crawler) accept(link string) bool {
	if link == "" {
		return false
	}
	if _, seen := c.visited[link]; seen {
		return false
	}
	for _, blk := range c.cfg.BlacklistedURLs {
		if strings.Contains(link, blk) {
			return false
		}
	}
	_, err := url.ParseRequestURI(link)
	return err == nil
}

// depthFirst walks one branch until MaxDepth or stop conditions fire.
func (c *Crawler) depthFirst(ctx context.Context, depth int) {
	if depth >= c.cfg.MaxDepth || ctx.Err() != nil || c.isTimeoutReached() {
		return
	}
	if len(c.links) == 0 {
		return
	}

	idx := c.rand.Intn(len(c.links))
	target := c.links[idx]
	c.links = append(c.links[:idx], c.links[idx+1:]...)
	c.visited[target] = struct{}{}

	body, err := c.fetch(ctx, target)
	if err != nil {
		log.Printf("visit %s: %v", target, err)
		return
	}

	c.links = append(c.links, c.extractLinks(body, target)...)

	sleep := time.Duration(c.rand.Intn(c.cfg.MaxSleep-c.cfg.MinSleep+1)+c.cfg.MinSleep) * time.Microsecond
	time.Sleep(sleep)

	c.depthFirst(ctx, depth+1)
}

func (c *Crawler) isTimeoutReached() bool {
	if c.cfg.Timeout == 0 {
		return false
	}
	return time.Since(c.startTime) > time.Duration(c.cfg.Timeout)*time.Second
}
