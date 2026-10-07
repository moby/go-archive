module github.com/moby/go-archive

go 1.25.0

require (
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20240806141605-e8a1dd7889d6
	github.com/containerd/log v0.1.0
	github.com/klauspost/compress v1.18.7
	github.com/moby/patternmatcher v0.6.1
	github.com/moby/sys/mount v0.3.5
	github.com/moby/sys/mountinfo v0.7.2
	github.com/moby/sys/reexec v0.1.0
	github.com/moby/sys/sequential v0.7.0
	github.com/moby/sys/user v0.4.1
	github.com/moby/sys/userns v0.1.0
	github.com/tonistiigi/fsutil v0.0.0-00010101000000-000000000000
	golang.org/x/sys v0.41.0
	gotest.tools/v3 v3.5.2
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/containerd/continuity v0.5.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	golang.org/x/sync v0.19.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/tonistiigi/fsutil => github.com/crazy-max/fsutil v0.0.0-20260818115129-c23945b4022f
