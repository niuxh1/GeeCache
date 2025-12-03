package httpclient

import (
	"fmt"
	pb "geecache/geecachepb"

	"io"
	"net/http"
	"net/url"

	"google.golang.org/protobuf/proto"
)

type HttpClient struct {
	BaseURL string
}

func (h *HttpClient) Get(in *pb.Request, out *pb.Response) error{
	u := fmt.Sprintf(
		"%v%v/%v",
		h.BaseURL,
		url.QueryEscape(in.GetGroup()),
		url.QueryEscape(in.GetKey()),
	)
    res, err := http.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}

	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}

	return nil
}