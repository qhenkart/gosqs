package example

import (
	"github.com/qhenkart/gosqs"
)

func main() {
	c := gosqs.Config{
		// for emulation only
		// Hostname: "http://localhost:4150",

		TopicARN: "arn:aws:sns:local:000000000000:dispatcher",
		Key:      "aws-key",
		Secret:   "aws-secret",
		Region:   "us-west-1",
	}

	//follows the flow to see how a worker should be configured and operate
	initWorker(c)

	//follows the flow to see how an http service should be configured and operate
	initService(c)

}

// Post defines a message post
type Post struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
	Body   string `json:"body"`
}

// ModelName satisfies the gosqs interface and allows messages to be sent out
//
// this will look like Dispatcher(ctx).Create(&post{})  will send the message   post_created
func (p Post) ModelName() string {
	return "post"
}
