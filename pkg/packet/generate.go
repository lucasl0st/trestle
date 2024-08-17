//go:build generate

package packet

//go:generate  protoc --proto_path=. --go_out=../ packet.proto
