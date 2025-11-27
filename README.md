# Simplistic Sigenergy solar inverter statistics gatherer

This is a very simple daemon to periodically poll a Sigenergy solar
inverter/battery system via MODBUS TCP to gather statistics.
Statistics are stored in a sqlite database.

The poller runs in the foreground and logs to syslog, SIGINT or SIGTERM
will gracefully terminate it. It doesn't require any privileges beyond
the ability to write to its database, so I recommend running it as an
unprivileged user.

It's written in Go, and can be built using `go build` after checkout.
Run `./solarpoller --help` for information about the (few) command-line
options.

There's also a Makefile, but that's mostly setup for my convenience.
E.g. it installs an OpenBSD rc.d init script. You might want to ignore
that.

This was written in a few hours for my specific needs, it's unlikely to be
adapted to yours. In particular, the polled variables are just the ones I
care about.
