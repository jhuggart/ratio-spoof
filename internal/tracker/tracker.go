package tracker

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/ap-pauloafonso/ratio-spoof/internal/bencode"
)

type HttpTracker struct {
	Urls               []string
	RetryAttempt       int
	LastAnounceRequest string
	LastTackerResponse string
}

type TrackerResponse struct {
	MinInterval int
	Interval    int
	Seeders     int
	Leechers    int
}

func NewHttpTracker(torrentInfo *bencode.TorrentInfo, timerChangeChannel chan<- int) (*HttpTracker, error) {

	var result []string
	for _, url := range torrentInfo.TrackerInfo.Urls {
		if strings.HasPrefix(url, "http") {
			result = append(result, url)
		}
	}
	if len(result) == 0 {
		return nil, errors.New("No tcp/http tracker url announce found")
	}
	return &HttpTracker{Urls: torrentInfo.TrackerInfo.Urls}, nil
}

func (T *HttpTracker) SwapFirst(currentIdx int) {
	aux := T.Urls[0]
	T.Urls[0] = T.Urls[currentIdx]
	T.Urls[currentIdx] = aux
}

func (T *HttpTracker) Announce(query string, headers map[string]string, retry bool, timerUpdateChannel chan<- int) (*TrackerResponse, error) {
	defer func() {
		T.RetryAttempt = 0
	}()
	if retry {
		retryDelay := 30 * time.Second
		for {
			trackerResp, err := T.tryMakeRequest(query, headers)
			if err != nil {
				timerUpdateChannel <- int(retryDelay.Seconds())
				T.RetryAttempt++
				time.Sleep(retryDelay)
				retryDelay *= 2
				if retryDelay.Seconds() > 900 {
					retryDelay = 900
				}
				continue
			}
			return trackerResp, nil
		}

	} else {
		resp, err := T.tryMakeRequest(query, headers)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
}

func (t *HttpTracker) tryMakeRequest(query string, headers map[string]string) (*TrackerResponse, error) {
	for idx, baseUrl := range t.Urls {
		completeURL := buildFullUrl(baseUrl, query)
		t.LastAnounceRequest = completeURL
		req, _ := http.NewRequest("GET", completeURL, nil)
		for header, value := range headers {
			req.Header.Add(header, value)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				bytesR, _ := ioutil.ReadAll(resp.Body)
				if len(bytesR) == 0 {
					continue
				}
				mimeType := http.DetectContentType(bytesR)
				if mimeType == "application/x-gzip" {
					gzipReader, _ := gzip.NewReader(bytes.NewReader(bytesR))
					bytesR, _ = ioutil.ReadAll(gzipReader)
					gzipReader.Close()
				}
				t.LastTackerResponse = string(bytesR)
				decodedResp, err := bencode.Decode(bytesR)
				if err != nil {
					continue
				}
				ret, err := extractTrackerResponse(decodedResp)
				if err != nil {
					continue
				}
				if idx != 0 {
					t.SwapFirst(idx)
				}

				return &ret, nil
			}
			resp.Body.Close()
		}
	}
	return nil, errors.New("Connection error with the tracker")

}

func buildFullUrl(baseurl, query string) string {
	if len(strings.Split(baseurl, "?")) > 1 {
		return baseurl + "&" + strings.TrimLeft(query, "&")
	}
	return baseurl + "?" + strings.TrimLeft(query, "?")
}

func extractTrackerResponse(datatrackerResponse map[string]interface{}) (TrackerResponse, error) {
	var result TrackerResponse
	if v, ok := datatrackerResponse["failure reason"].(string); ok && len(v) > 0 {
		return result, errors.New(v)
	}
	result.MinInterval, _ = datatrackerResponse["min interval"].(int)
	result.Interval, _ = datatrackerResponse["interval"].(int)
	result.Seeders, _ = datatrackerResponse["complete"].(int)
	result.Leechers, _ = datatrackerResponse["incomplete"].(int)
	return result, nil

}
