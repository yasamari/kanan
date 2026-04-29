package util

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"sync"
)

type CacheTransport struct {
	Base  http.RoundTripper
	cache sync.Map
}

func (t *CacheTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 1. キャッシュがあればそれを返す
	url := req.URL.String()
	if val, ok := t.cache.Load(url); ok {
		body := val.([]byte)
		slog.Debug("Cache hit", "url", url)
		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          io.NopCloser(bytes.NewReader(body)),
			ContentLength: int64(len(body)),
			Header:        make(http.Header),
			Request:       req,
		}, nil
	}

	// 2. キャッシュがなければ実際のリクエストを実行
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// 3. レスポンスボディを読み取ってキャッシュに保存
	// (Bodyは一度しか読めないので、読み取った後に差し替える必要がある)
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	t.cache.Store(url, bodyBytes)

	// 元のレスポンスと同じ内容を呼び出し元に返す
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	slog.Debug("Cache miss", "url", url)
	return resp, nil
}
