INSTALL=install -o root -g wheel

all: solarpoller

solarpoller: solarpoller.go
	go build

clean:
	rm solarpoller

install:
	$(INSTALL) -d -m 0755 /usr/local/sbin
	$(INSTALL) -m 0755 solarpoller /usr/local/sbin
	$(INSTALL) -m 0555 solarpoller.rc /etc/rc.d/solarpoller
	id -u _solarpoller >/dev/null 2>&1 || \
		useradd -d /var/empty -r 900..999 -s /sbin/nologin  _solarpoller
	test -d /var/db/readings || \
		mkdir -m 0775 /var/db/readings
	test -f /var/db/readings/readings.sqlite || \
		./mkdb.sh /var/db/readings/readings.sqlite
	chown _solarpoller.wheel /var/db/readings
	chown _solarpoller.wheel /var/db/readings/readings.sqlite
	chmod 0775 /var/db/readings
	chmod 0660 /var/db/readings/readings.sqlite

