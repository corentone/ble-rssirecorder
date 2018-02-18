package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/examples/lib/dev"
	"github.com/pkg/errors"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

var (
	device = flag.String("device", "default", "implementation of ble")
	du     = flag.Duration("du", 5*time.Second, "scanning duration")
	dup    = flag.Bool("dup", true, "allow duplicate reported")
)

var valueByName map[string][]Row

func main() {
	flag.Parse()

	done := make(chan bool)
	gotValues := make(chan BLEDevice, 100)
	go startAcquirer(gotValues, done)

	d, err := dev.NewDevice(*device)
	if err != nil {
		log.Fatalf("can't new device : %s", err)
	}
	ble.SetDefaultDevice(d)

	ctx, cancelFunc := context.WithCancel(context.Background())
	ctx = ble.WithSigHandler(ctx, func() {
		cancelFunc()
		close(gotValues)
	})

	fmt.Printf("Starting... Interrupt with Ctrl+C\n")
	chkErr(ble.Scan(ctx, *dup, createAdvHandler(gotValues), nil))
	<-done
	printSummary()
}

func adPrettyPrint(a ble.Advertisement) {
	fmt.Printf("Name: %v | TxPowerLevel: %v\n", a.LocalName(), a.TxPowerLevel())
	fmt.Printf("    RSSI: %v | Addr: %v\n", a.RSSI(), a.Addr().String())
}

func rows2PlotterXYS(rows []Row) plotter.XYs {
	pts := make(plotter.XYs, len(rows))
	for i, row := range rows {
		pts[i].X = float64(row.T.UnixNano()-rows[0].T.UnixNano()) / 1000000000
		pts[i].Y = float64(row.RSSI)
	}
	return pts
}

func displayImage(filename string) {
	err := exec.Command(
		"open", "-a", "/Applications/Preview.app", filename,
	).Run()

	if err != nil {
		panic(err)
	}
}

func printSummary() {
	fmt.Printf("----Summary---\n")
	fmt.Printf("%+v\n", valueByName)

	for name, rows := range valueByName {
		fmt.Printf("Graph for Device %v; Got %v Samples over %v secondes\n", name, len(rows), rows[len(rows)-1].T.Unix()-rows[0].T.Unix())

		p, err := plot.New()
		if err != nil {
			panic(err)
		}
		p.Title.Text = "RSSI over time"
		p.X.Label.Text = "time(s)"
		p.Y.Label.Text = "RSSI(dBm)"

		err = plotutil.AddLinePoints(p,
			name, rows2PlotterXYS(rows),
		)
		if err != nil {
			panic(err)
		}

		// Save the plot to a PNG file.
		filename := "./points_" + name + ".png"
		err = p.Save(8*vg.Inch, 8*vg.Inch, filename)
		if err != nil {
			panic(err)
		}
		displayImage(filename)

	}
	fmt.Printf("---/Summary---\n")
}

type BLEDevice struct {
	Name string
	RSSI int
	T    time.Time
}

type Row struct {
	T    time.Time
	RSSI int
}

func startAcquirer(gotValues chan BLEDevice, done chan bool) {
	valueByName = make(map[string][]Row)
	for device := range gotValues {
		valueByName[device.Name] = append(valueByName[device.Name], Row{T: device.T, RSSI: device.RSSI})
	}
	close(done)
}

func createAdvHandler(c chan BLEDevice) func(a ble.Advertisement) {
	return func(a ble.Advertisement) {
		if strings.Contains(a.LocalName(), "Co") && a.LocalName() != "" {
			//adPrettyPrint(a)
			c <- BLEDevice{Name: a.LocalName(), RSSI: a.RSSI(), T: time.Now()}
		}
		/*
			if a.Connectable() {
				fmt.Printf("[%s] C %3d:", a.Addr(), a.RSSI())
			} else {
				fmt.Printf("[%s] N %3d:", a.Addr(), a.RSSI())
			}
			comma := ""
			if len(a.LocalName()) > 0 {
				fmt.Printf(" Name: %s", a.LocalName())
				comma = ","
			}
			if len(a.Services()) > 0 {
				fmt.Printf("%s Svcs: %v", comma, a.Services())
				comma = ","
			}
			if len(a.ManufacturerData()) > 0 {
				fmt.Printf("%s MD: %X", comma, a.ManufacturerData())
			}
			fmt.Printf("\n")
		*/
	}
}

func chkErr(err error) {
	switch errors.Cause(err) {
	case nil:
	case context.DeadlineExceeded:
		fmt.Printf("done\n")
	case context.Canceled:
		fmt.Printf("canceled\n")
	default:
		log.Fatalf(err.Error())
	}
}
