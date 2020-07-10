package example

import (
	"net/http"

	"github.com/qhenkart/gosqs"
)

// ServiceHandler example http handler
type ServiceHandler struct{}

// add the publisher as middleware
func httpMiddleware(pub gosqs.Publisher, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := gosqs.WithDispatcher(r.Context(), pub)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func initService(c gosqs.Config) {
	// create the connection to AWS or the emulator
	pub, err := gosqs.NewPublisher(c)
	if err != nil {
		panic(err)
	}
	h := &ServiceHandler{}

	http.Handle("/", httpMiddleware(pub, http.HandlerFunc(h.Test)))
}

// Test a showcase of various ways to send messages
func (h *ServiceHandler) Test(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	p := Post{}

	disp := gosqs.MustDispatcher(ctx)

	// these methods always have the modelname prepended
	// they are sent to the notifier which then is sent to every queue.
	// if there is no event listener, they will be discarded in that queue

	// post_created
	disp.Create(&p)
	// post_deleted
	disp.Delete(&p)
	// post_updated
	disp.Update(&p)
	// post_some_other_event
	disp.Dispatch(&p, "some_other_event")

	// modified helper
	changes := map[string]string{
		"body": "original body",
	}

	p.Body = "new body"

	// post_modified   includes both the post body as well as a changes map
	disp.Modify(&p, changes)

	//direct messages will only be sent to the specific queue and will skip the SNS entirely
	disp.Message("post-worker", "custom_message", &p)
}
