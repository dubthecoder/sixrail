module github.com/teclara/railsix/gtfs-static

go 1.25.8

require (
	github.com/jamespfennell/gtfs v0.1.24
	github.com/teclara/railsix/shared v0.0.0
)

require (
	github.com/google/go-cmp v0.6.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
)

replace github.com/teclara/railsix/shared => ../shared
