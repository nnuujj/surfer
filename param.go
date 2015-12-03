package surfer

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/henrylee2cn/surfer/agent"
	"github.com/henrylee2cn/surfer/util"
)

type Param struct {
	method        string
	originUrl     *url.URL
	url           *url.URL
	proxy         *url.URL
	contentType   string
	body          io.Reader
	header        http.Header
	enableCookie  bool
	dialTimeout   time.Duration
	connTimeout   time.Duration
	tryTimes      int
	retryPause    time.Duration
	redirectTimes int
	client        *http.Client
}

func NewParam(req Request) (param *Param, err error) {
	param = new(Param)
	param.originUrl, err = url.Parse(req.GetUrl())
	if err != nil {
		return nil, err
	}

	param.url, _ = util.UrlEncode(req.GetUrl())

	if req.GetProxy() != "" {
		if param.proxy, err = url.Parse(req.GetProxy()); err != nil {
			return nil, err
		}
	}

	switch method := strings.ToUpper(req.GetMethod()); method {
	case "GET", "HEAD":
		param.method = method
	case "POST":
		param.method = method
		param.contentType = "application/x-www-form-urlencoded"
		param.body = strings.NewReader(req.GetPostData().Encode())
	case "POST-M":
		param.method = "POST"
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		for k, vs := range req.GetPostData() {
			for _, v := range vs {
				writer.WriteField(k, v)
			}
		}
		err := writer.Close()
		if err != nil {
			return nil, err
		}
		param.contentType = writer.FormDataContentType()
		param.body = body

	default:
		param.method = "GET"
	}

	param.header = make(http.Header)

	if param.contentType != "" {
		param.header.Set("Content-Type", param.contentType)
	}

	for k, v := range req.GetHeader() {
		for _, vv := range v {
			param.header.Add(k, vv)
		}
	}

	param.enableCookie = req.GetEnableCookie()

	if len(param.header.Get("User-Agent")) == 0 {
		if param.enableCookie {
			param.header.Set("User-Agent", agent.UserAgents["common"][0])
		} else {
			l := len(agent.UserAgents["common"])
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			param.header.Set("User-Agent", agent.UserAgents["common"][r.Intn(l)])
		}
	}

	param.dialTimeout = req.GetDialTimeout()
	if param.dialTimeout < 0 {
		param.dialTimeout = 0
	}

	param.connTimeout = req.GetConnTimeout()
	param.tryTimes = req.GetTryTimes()
	param.retryPause = req.GetRetryPause()
	param.redirectTimes = req.GetRedirectTimes()
	return
}

// 回写Request内容
func (self *Param) writeback(resp *http.Response) {
	resp.Request.URL = self.originUrl
	resp.Request.Method = self.method
	resp.Request.Header = self.header
	resp.Request.Host = self.url.Host
}

// checkRedirect is used as the value to http.Client.CheckRedirect
// when redirectTimes equal 0, redirect times is ∞
// when redirectTimes less than 0, not allow redirects
func (self *Param) checkRedirect(req *http.Request, via []*http.Request) error {
	if self.redirectTimes == 0 {
		return nil
	}
	if len(via) >= self.redirectTimes {
		if self.redirectTimes < 0 {
			return fmt.Errorf("not allow redirects.")
		}
		return fmt.Errorf("stopped after %v redirects.", self.redirectTimes)
	}
	return nil
}
