module mindrot.org/solarpoller

go 1.25.1

require (
	github.com/mattn/go-sqlite3 v1.14.32
	github.com/simonvetter/modbus v1.6.4
)

require github.com/goburrow/serial v0.1.1-0.20211022031912-bfb69110f8dd // indirect

replace github.com/simonvetter/modbus => github.com/djmdjm/modbus v0.0.0-20251126105348-7107ddcf7fa4
