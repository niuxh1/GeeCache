package httpserver

import (
	"fmt"
	group "geecache/Group"
	pb "geecache/geecachepb"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
)

func (p *HttpAddr) Log(format string, v ...interface{}) {
	log.Printf("[Serve on %s] %s", p.Path, fmt.Sprintf(format, v...))
}

func (p *HttpAddr) Serve(c *gin.Context) {
	if !strings.HasPrefix(c.Request.URL.Path, p.Path) {
		panic(fmt.Sprintf("GeeCache get unexcepted path : %s", c.Request.URL.Path))
	}
	p.Log("Received %s request: %s", c.Request.Method, c.Request.URL.Path)

	parts := strings.SplitN(c.Request.URL.Path[len(p.Path):], "/", 2)
	if len(parts) != 2 {
		c.String(
			400,
			"Bad Request",
		)
		return
	}
	// Path/GroupName/Key
	groupName := parts[0]
	key := parts[1]

	group := group.GetGroup(groupName)
	if group == nil {
		c.String(
			404,
			"Group Not Found",
		)
		return
	}

	bv, err := group.Get(key)
	if err != nil {
		c.String(
			500,
			err.Error(),
		)
		return
	}

	body, err := proto.Marshal(&pb.Response{Value: bv.ByteSlice()})
	if err != nil {
		c.String(
			500,
			err.Error(),
		)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Data(200, "application/octet-stream", body)
}
