module github.com/my/repo

go 1.13

replace emu => ../src/emu

replace external => ../src/external

replace external/google/gopacket => ../src/external/google/gopacket

require (
	emu v0.0.0-00010101000000-000000000000 // indirect
	external v0.0.0-00010101000000-000000000000 // indirect
	external/google/gopacket v0.0.0-00010101000000-000000000000 // indirect
	github.com/alecthomas/jsonschema v0.0.0-20191017121752-4bb6e3fae4f2 // indirect
)
