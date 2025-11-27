#!/bin/sh

set -xe

DB=db.sqlite
test -z "$1" || DB="$1"

cat << _EOF | sqlite3 $DB
CREATE TABLE IF NOT EXISTS readings (
	ts DATETIME NOT NULL,
	device TEXT NOT NULL,
	sensor TEXT NOT NULL,
	valueFloat REAL,
	valueInt INTEGER
);
_EOF

cat << _EOF | sqlite3 $DB
CREATE INDEX IF NOT EXISTS idx_ts ON readings (ts);
CREATE INDEX IF NOT EXISTS idx_device ON readings (device);
CREATE INDEX IF NOT EXISTS idx_sensor ON readings (sensor);
CREATE INDEX IF NOT EXISTS idx_device_sensor ON readings (device, sensor);
_EOF
