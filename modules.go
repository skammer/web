package web

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
)

/**
Generic {json,xml} encoding interface
*/
type Encoder interface {
	Encode(interface{}) error
}

/**
This is really only useful for specific requests and should probably
be thought out a little more. If a client accepts * / *, how do we
determine what the best Content-Type is for the particular data?
Right now we would just send the original content
*/
func MarshalResponse(ctx *Context, content interface{}) (interface{}, error) {
	var encoded bytes.Buffer
	var enc Encoder

	ctx.SetHeader("Content-Type", "text/plain", true)
	if len(ctx.Request.Header["Accept"]) > 0 {
		for _, accepts := range ctx.Request.Header["Accept"] {

			// Look for a JSON request
			if strings.Index(accepts, "application/json") >= 0 {
				enc = json.NewEncoder(&encoded)
				ctx.SetHeader("Content-Type", "application/json", true)
			}

			// Look for an XML request
			if strings.Index(accepts, "text/xml") >= 0 ||
				strings.Index(accepts, "application/xml") >= 0 {
				if reflect.TypeOf(content).Kind() == reflect.Map {
					ctx.NotAcceptable("Can not encode datatype")
					err := WebError{"Can not encode datatype"}
					return content, err
				}
				enc = xml.NewEncoder(&encoded)
				ctx.SetHeader("Content-Type", "text/xml", true)
			}
			if strings.Index(accepts, "video/mp4") >= 0 {
				// Setup the connection to receive a vdeo stream
				ctx.SetHeader("Content-Type", "video/mp4", true)
				ctx.SetHeader("Content-Disposition", "inline; filename=motion.mpeg4", true)

				// We should be passed a File pointer to stream from
				var c *os.File
				t := reflect.TypeOf(content)
				if t.Kind() != reflect.Ptr || t.Elem().String() != "os.File" {
                    ctx.Abort(500, "Can not stream data source")
					return nil, WebError{"Can not stream data source"}
				}

                // Hand out the correct size
				c = content.(*os.File)
				stat, _ := c.Stat()
				ctx.SetHeader("Content-Length", fmt.Sprintf("%d", stat.Size()), true)

                // Do a buffer copy, which will stream the data to the client
				io.Copy(ctx.ResponseWriter, c)
				return nil, nil
			}
		}
	}

	if enc != nil {
		err := enc.Encode(content)
		if err != nil {
			return content, err
		}
		return encoded.Bytes(), nil
	}

	// If we don't have a MIME type handler, just return the
	// original content
	return content, nil
	return []byte(content.(string)), nil

}

/**
Attempts to encode the response according to the client's Accept-Encoding
header. If there is an error, or if the encoding requests aren't supported
then the original content is returned.

Encoding type:
 * deflate (zlib stream)
 * gzip

This should be the last module loaded
*/
func EncodeResponse(ctx *Context, content interface{}) (interface{}, error) {
	var compressed bytes.Buffer
	var output io.WriteCloser

	if len(ctx.Request.Header["Accept-Encoding"]) > 0 {
		for _, opt := range ctx.Request.Header["Accept-Encoding"] {
			if strings.Index(opt, "gzip") >= 0 {
				output = gzip.NewWriter(&compressed)
				ctx.SetHeader("Content-Encoding", "gzip", true)
			}
			if strings.Index(opt, "deflate") >= 0 {
				output = zlib.NewWriter(&compressed)
				ctx.SetHeader("Content-Encoding", "deflate", true)
			}
		}
	}

	if output != nil {
		_, err := output.Write(content.([]byte))
		if err != nil {
			return content, err
		}
		err = output.Close()
		return compressed.Bytes(), nil
	}

	return content, nil
}
