// The proof that the native table asks what the web table asks.
//
// It is a module of its own, as module/ and catalogue/ are, because it is the
// only place that holds Ayra and Kyse in one graph: the drawing half must not
// depend on the framework (see dependency_test.go at the root), and Kyse is what
// the state is compared with. Nothing imports it and nothing it holds is shipped.
module github.com/arandu-io/ayra/compat

go 1.26.10

require (
	github.com/arandu-io/ayra v0.0.0
	github.com/arandu-io/hesape v0.89.2
	github.com/arandu-io/kyse v0.46.0
)

require (
	github.com/go-text/typesetting v0.3.5 // indirect
	golang.org/x/image v0.46.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

replace github.com/arandu-io/ayra => ..
