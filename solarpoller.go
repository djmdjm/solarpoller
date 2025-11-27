package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/simonvetter/modbus"
)

type RegType int
type ValType int

const (
	U16 = RegType(iota)
	S16
	U32
	S32
	U64
	Float = ValType(iota)
	StatusWord
)

var (
	target       = flag.String("target", "", "MODBUS target, e.g. tcp://192.168.0.1:502")
	timeout      = flag.Int("timeout", 1000, "MODBUS timeout in milliseconds")
	database     = flag.String("database", "db.sqlite", "Path to database")
	deviceID     = flag.String("device-identifier", "solar", "Device identifier in database")
	intervalSpec = flag.String("interval", "5m", "Interval between polls")
	logToStderr  = flag.Bool("logtostderr", false, "Log to stderr instead of syslog")
	debug        = flag.Bool("debug", false, "Log debugging information")
	once         = flag.Bool("once", false, "Run once (i.e. don't poll)")
	dryRun       = flag.Bool("dry-run", false, "Run once in debug mode and don't update database")
)

type ReadVar struct {
	Name         string
	UnitID       uint8
	Register     uint16
	RegisterType RegType
	ValueType    ValType
	Scale        float64
	Unit         string
}

var readVars = []ReadVar{
	ReadVar{"status_on_grid", 247, 30009, U16, StatusWord, 0, ""},
	ReadVar{"status_alarm1_pcs", 247, 30027, U16, StatusWord, 0, ""},
	ReadVar{"status_alarm2_pcs", 247, 30028, U16, StatusWord, 0, ""},
	ReadVar{"status_alarm3_ess", 247, 30029, U16, StatusWord, 0, ""},
	ReadVar{"status_alarm4_gateway", 247, 30030, U16, StatusWord, 0, ""},
	ReadVar{"status_alarm5_dc_charger", 247, 30030, U16, StatusWord, 0, ""},
	ReadVar{"status_running_state", 1, 30578, U16, StatusWord, 0, ""},
	ReadVar{"power_grid_active", 247, 30005, S32, Float, 0.001, "kW"},
	ReadVar{"power_grid_reactive", 247, 30007, S32, Float, 0.001, "Kvar"},
	ReadVar{"ess_battery_charge_percent", 247, 30014, U16, Float, 0.1, "%"},
	ReadVar{"power_plant_active", 247, 30031, S32, Float, 0.001, "kW"},
	ReadVar{"power_plant_reactive", 247, 30033, S32, Float, 0.001, "Kvar"},
	ReadVar{"power_pv", 247, 30035, S32, Float, 0.001, "kW"},
	ReadVar{"power_ess", 247, 30037, S32, Float, 0.001, "kW"},
	ReadVar{"energy_ess_capacity_charge", 247, 30064, U32, Float, 0.01, "kWh"},
	ReadVar{"energy_ess_capacity_discharge", 247, 30066, U32, Float, 0.01, "kWh"},
	ReadVar{"capacity_ess_health", 247, 30083, U32, Float, 0.01, "kWh"},
	ReadVar{"ess_battery_health_percent", 247, 30087, U16, Float, 0.1, "%"},
	ReadVar{"temperature_ess_cell_avg", 1, 30603, S16, Float, 0.1, "C"},
	ReadVar{"energy_daily_ess_charge", 1, 30566, U32, Float, 0.01, "kWh"},
	ReadVar{"energy_accum_ess_charge", 1, 30568, U64, Float, 0.01, "kWh"},
	ReadVar{"energy_daily_ess_discharge", 1, 30572, U32, Float, 0.01, "kWh"},
	ReadVar{"energy_accum_ess_discharge", 1, 30574, U64, Float, 0.01, "kWh"},
	ReadVar{"voltage_ess_battery_avg", 1, 30604, S16, Float, 0.001, "V"},
	ReadVar{"temperature_ess_cluster_max", 1, 30620, S16, Float, 0.1, "C"},
	ReadVar{"temperature_ess_cluster_min", 1, 30621, S16, Float, 0.1, "C"},
	ReadVar{"frequency_grid", 1, 31002, U16, Float, 0.01, "Hz"},
	ReadVar{"temperature_pcs", 1, 31003, S16, Float, 0.1, "C"},
	ReadVar{"voltage_line_ab", 1, 31005, U32, Float, 0.01, "V"},
	ReadVar{"voltage_line_bc", 1, 31007, U32, Float, 0.01, "V"},
	ReadVar{"voltage_line_ca", 1, 31009, U32, Float, 0.01, "V"},
	ReadVar{"voltage_phase_a", 1, 31011, U32, Float, 0.01, "V"},
	ReadVar{"voltage_phase_b", 1, 31013, U32, Float, 0.01, "V"},
	ReadVar{"voltage_phase_c", 1, 31015, U32, Float, 0.01, "V"},
	ReadVar{"current_phase_a", 1, 31017, S32, Float, 0.01, "A"},
	ReadVar{"current_phase_b", 1, 31019, S32, Float, 0.01, "A"},
	ReadVar{"current_phase_c", 1, 31021, S32, Float, 0.01, "A"},
	ReadVar{"power_factor", 1, 31023, U16, Float, 0.001, ""},
	ReadVar{"voltage_pv1", 1, 31027, S16, Float, 0.1, "V"},
	ReadVar{"current_pv1", 1, 31028, S16, Float, 0.01, "A"},
	ReadVar{"voltage_pv2", 1, 31029, S16, Float, 0.1, "V"},
	ReadVar{"current_pv2", 1, 31030, S16, Float, 0.01, "A"},
	ReadVar{"voltage_pv3", 1, 31031, S16, Float, 0.1, "V"},
	ReadVar{"current_pv3", 1, 31032, S16, Float, 0.01, "A"},
	ReadVar{"voltage_pv4", 1, 31033, S16, Float, 0.1, "V"},
	ReadVar{"current_pv4", 1, 31034, S16, Float, 0.01, "A"},
	ReadVar{"power_pv", 1, 31035, S32, Float, 0.001, "kW"},
	ReadVar{"resistance_insulation", 1, 31037, U16, Float, 0.001, "MΩ"},
	ReadVar{"energy_accum_pv", 247, 30088, U64, Float, 0.01, "kWh"},
	ReadVar{"energy_daily_consumed", 247, 30092, U32, Float, 0.01, "kWh"},
	ReadVar{"energy_accum_consumed", 247, 30094, U64, Float, 0.01, "kWh"},
	ReadVar{"energy_accum_battery_discharge", 247, 30204, U64, Float, 0.01, "kWh"},
	ReadVar{"energy_accum_grid_import", 247, 30216, U64, Float, 0.01, "kWh"},
	ReadVar{"energy_accum_grid_export", 247, 30220, U64, Float, 0.01, "kWh"},
	ReadVar{"energy_total_load_consumed", 247, 30228, U64, Float, 0.01, "kWh"},
	ReadVar{"energy_total_pv_generated", 247, 30236, U64, Float, 0.01, "kWh"},
}

func readRegisterFloat(client *modbus.ModbusClient, registerID uint16, registerType RegType) (float64, error) {
	switch registerType {
	case U16:
		reg16, err := client.ReadRegister(registerID, modbus.HOLDING_REGISTER)
		if err != nil {
			return 0, err
		}
		return float64(reg16), nil
	case S16:
		reg16, err := client.ReadRegister(registerID, modbus.HOLDING_REGISTER)
		if err != nil {
			return 0, err
		}
		return float64(int16(reg16)), nil
	case U32:
		reg32s, err := client.ReadUint32s(registerID, 1, modbus.HOLDING_REGISTER)
		if err != nil {
			return 0, err
		}
		return float64(reg32s[0]), nil
	case S32:
		reg32s, err := client.ReadUint32s(registerID, 1, modbus.HOLDING_REGISTER)
		if err != nil {
			return 0, err
		}
		return float64(int32(reg32s[0])), nil
	case U64:
		reg64s, err := client.ReadUint64s(registerID, 1, modbus.HOLDING_REGISTER)
		if err != nil {
			return 0, err
		}
		return float64(reg64s[0]), nil
	default:
		return 0, fmt.Errorf("unsupported register type %v", registerType)
	}
}

func readRegisterStatusWord(client *modbus.ModbusClient, registerID uint16, registerType RegType) (uint32, error) {
	switch registerType {
	case U16:
		reg16, err := client.ReadRegister(registerID, modbus.HOLDING_REGISTER)
		if err != nil {
			return 0, err
		}
		return uint32(reg16), nil
	case U32:
		reg32s, err := client.ReadUint32s(registerID, 1, modbus.HOLDING_REGISTER)
		if err != nil {
			return 0, err
		}
		return reg32s[0], nil
	default:
		return 0, fmt.Errorf("unsupported register type %v", registerType)
	}
}

func readAll(client *modbus.ModbusClient) (map[string]float64, map[string]uint32, error) {
	values := map[string]float64{}
	statuses := map[string]uint32{}
	for _, rv := range readVars {
		client.SetUnitId(rv.UnitID)
		switch rv.ValueType {
		case Float:
			result, err := readRegisterFloat(client, rv.Register, rv.RegisterType)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read %v (%v): %v", rv.Name, rv.Register, err)
			}
			result *= rv.Scale
			values[rv.Name] = result
			if *debug {
				log.Printf("%v: %.3f %v", rv.Name, result, rv.Unit)
			}
		case StatusWord:
			result, err := readRegisterStatusWord(client, rv.Register, rv.RegisterType)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read %v (%v): %v", rv.Name, rv.Register, err)
			}
			statuses[rv.Name] = result
			if *debug {
				log.Printf("%v: 0x%04x", rv.Name, result)
			}
		}
	}
	return values, statuses, nil
}

func poll(client *modbus.ModbusClient, db *sql.DB, now time.Time) error {
	err := client.Open()
	if err != nil {
		return err
	}
	defer client.Close()
	values, statuses, err := readAll(client)
	if err != nil {
		return fmt.Errorf("read values: %v", err)
	}
	if *dryRun {
		return nil
	}
	for k, v := range statuses {
		_, err = db.Exec("INSERT INTO readings(ts, device, sensor, valueInt) VALUES (?, ?, ?, ?)", now, *deviceID, k, v)
		if err != nil {
			return fmt.Errorf("insert status: %v", err)
		}
	}
	if *debug {
		log.Printf("wrote %v status readings", len(statuses))
	}
	for k, v := range values {
		_, err = db.Exec("INSERT INTO readings(ts, device, sensor, valueFloat) VALUES (?, ?, ?, ?)", now, *deviceID, k, v)
		if err != nil {
			return fmt.Errorf("insert value: %v", err)
		}
	}
	if *debug {
		log.Printf("wrote %v value readings", len(values))
	}
	return nil
}

func mainloop(interval time.Duration, client *modbus.ModbusClient, db *sql.DB) {
	// Loop until terminated by signal.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case s := <-sigint:
			log.Printf("Exit: %v", s)
			return
		case now := <-ticker.C:
			err := poll(client, db, now)
			if err != nil {
				log.Print(err)
			}
		}
	}
}

func main() {
	flag.Parse()
	interval, err := time.ParseDuration(*intervalSpec)
	if err != nil {
		log.Fatalf("invalid --interval: %v", err)
	}
	if *target == "" {
		log.Fatalf("Error: --target not specified")
	}
	if *dryRun {
		log.SetFlags(0)
		*once = true
		*debug = true
		*logToStderr = true
	}
	db, err := sql.Open("sqlite3", *database)
	if err != nil {
		log.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	client, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL:     *target,
		Timeout: time.Duration(*timeout) * time.Millisecond,
	})
	if err != nil {
		log.Fatalf("Failed to create MODBUS client: %v", err)
	}
	if !*logToStderr {
		progname := filepath.Base(os.Args[0])
		syslogger, err := syslog.Dial("", "", syslog.LOG_ERR|syslog.LOG_DAEMON, progname)
		if err != nil {
			log.Fatalf("Couldn't prepare syslog: %v", err)
		}
		defer syslogger.Close()
		log.SetOutput(syslogger)
	}
	// time.Ticker doesn't fire initially, so run once manually to start.
	err = poll(client, db, time.Now())
	if err != nil {
		if *once {
			log.Fatal(err)
		}
		log.Print(err)
	}
	if !*once {
		log.Printf("started; will poll %v every %v and write to %v", *target, interval, *database)
		mainloop(interval, client, db)
	}
}
