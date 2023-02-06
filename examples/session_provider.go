package example

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/qhenkart/gosqs"
)

func main_with_session_provider() {

	// implement a custom AWS session provider function
	provider := func(c gosqs.Config) (*session.Session, error) {

		// note: this implementation just hardcodes key and secret, but it could do anything
		creds := credentials.NewStaticCredentials("mykey", "mysecret", "")
		_, err := creds.Get()
		if err != nil {
			return nil, gosqs.ErrInvalidCreds.Context(err)
		}

		cfg := aws.NewConfig().WithRegion("us-west-1").WithCredentials(creds)

		hostname := "http://localhost:4150"
		cfg.Endpoint = &hostname

		return session.NewSession(cfg)
	}

	// create the gosqs Config with our custom SessionProviderFunc
	c := gosqs.Config{
		// for emulation only
		// Hostname: "http://localhost:4150",

		SessionProvider: provider,
		TopicARN:        "arn:aws:sns:local:000000000000:dispatcher",
		Region:          "us-west-1",
	}

	//follows the flow to see how a worker should be configured and operate
	initWorker(c)

	//follows the flow to see how an http service should be configured and operate
	initService(c)

}
