package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/farjump/go-libudev"
)


func runcmd(cmdStr string) {
	if cmdStr == "" {
		return
	}
	cmd := exec.Command("/bin/sh", "-c", cmdStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Println("Running", cmdStr)
	cmd.Run()
}

func main() {
	usbvendor := flag.String("v", "0bda", "usb vendor")
	usbproduct := flag.String("p", "1100", "usb product")
	attachCmd := flag.String("attach", "echo Attached.", "command to run on attach")
	detachCmd := flag.String("detach", "echo Detached.", "command to run on detach")

	flag.Parse()

	vendornum, err := strconv.ParseUint(*usbvendor, 16, 32)
	if err != nil {
		log.Fatal("error parsing -v:", err)
	}
	productnum, err := strconv.ParseUint(*usbproduct, 16, 32)
	if err != nil {
		log.Fatal("error parsing -p:", err)
	}

	u := udev.Udev{}
	devEnum := u.NewEnumerate()
	devmon := u.NewMonitorFromNetlink("kernel")

	// Set up the monitor, to wait for detach/attach events
	err = devmon.FilterAddMatchSubsystem("hidraw")
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deviceCh, err := devmon.DeviceChan(ctx)
	if err != nil {
		log.Fatal(err)
	}

	product := fmt.Sprintf("%x/%x/", vendornum, productnum)
	matchDevice := func(d *udev.Device) bool {
			usbParent := d.ParentWithSubsystemDevtype("usb", "usb_device")

			return usbParent != nil && strings.HasPrefix(usbParent.Properties()["PRODUCT"], product)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		log.Println("Starting monitor")


		for d := range deviceCh {

			if !matchDevice(d) {
				continue
			}

			switch d.Action() {
			case "add":
				runcmd(*attachCmd)
			case "remove":
				runcmd(*detachCmd)
			}

		}
		wg.Done()
	}()


	// Set up the enumerator, to get the current status
	err = devEnum.AddMatchSubsystem("hidraw")
	if err != nil {
		log.Fatal(err)
	}
	devEnum.AddMatchIsInitialized()
	if err != nil {
		log.Fatal(err)
	}

	matched := false

	devices, err := devEnum.Devices()
	if err != nil {
		log.Fatal(err)
	}

	for _, d := range devices {
		if matchDevice(d) {
			matched = true
			break
		}
	}

	// A bit of a race condition here, but it's fine.
	if matched {
		runcmd(*attachCmd)
	} else {
		runcmd(*detachCmd)
	}

	wg.Wait()

}
