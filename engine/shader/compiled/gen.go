package compiled

// Shader assets are checked in. Keep their generated Go bindings target-aware
// after refreshing shaders.go.
//go:generate go run ./cmd/postprocessshaders -file shaders.go
