package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	config "github.com/c3os-io/c3os/cli/config"
	"github.com/c3os-io/c3os/cli/utils"

	machine "github.com/c3os-io/c3os/cli/machine"
	nodepair "github.com/mudler/go-nodepair"
	qr "github.com/mudler/go-nodepair/qrcode"
	"github.com/pterm/pterm"
	"gopkg.in/yaml.v2"
)

func optsToArgs(options map[string]string) (res []string) {
	for k, v := range options {
		if k != "device" && k != "cc" && k != "reboot" && k != "poweroff" {
			res = append(res, fmt.Sprintf("--%s", k))
			res = append(res, fmt.Sprintf("%s", v))
		}
	}
	return
}

func install(dir ...string) error {
	tk := ""

	// Reads config, and if present and offline is defined,
	// runs the installation
	cc, err := config.Scan(dir...)
	if err == nil && cc.C3OS != nil && cc.C3OS.Offline {
		runInstall(map[string]string{
			"device": cc.C3OS.Device,
			"cc":     cc.String(),
		})

		svc, err := machine.Getty(1)
		if err == nil {
			svc.Start()
		}

		return nil
	} else if err == nil && cc.C3OS != nil && cc.C3OS.NetworkToken != "" {
		tk = cc.C3OS.NetworkToken
	}

	utils.PrintBanner(banner)
	if tk == "" {
		tk = nodepair.GenerateToken()
	}

	pterm.DefaultBox.WithTitle("Installation").WithTitleBottomRight().WithRightPadding(0).WithBottomPadding(0).Println(
		`Welcome to c3os!
p2p device installation enrollment is starting.
A QR code will be displayed below. 
In another machine, run "c3os register" with the QR code visible on screen,
or "c3os register <file>" to register the machine from a photo.
IF the qrcode is not displaying correctly,
try booting with another vga option from the boot cmdline (e.g. vga=791).`)

	pterm.Info.Println("Press any key to abort pairing. To restart run 'c3os install'.")

	pterm.Info.Println("Starting in 5 seconds...")
	pterm.Print("\n\n") // Add two new lines as spacer.

	time.Sleep(5 * time.Second)

	qr.Print(tk)

	r := map[string]string{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		// Wait for user input and go back to shell
		utils.Prompt("")
		cancel()
		// give tty1 back
		svc, err := machine.Getty(1)
		if err == nil {
			svc.Start()
		}
	}()

	if err := nodepair.Receive(ctx, &r, nodepair.WithToken(tk)); err != nil {
		return err
	}

	if len(r) == 0 {
		return errors.New("no configuration, stopping installation")
	}

	pterm.Info.Println("Starting installation")

	runInstall(r)

	pterm.Info.Println("Installation completed, press enter to go back to the shell.")

	utils.Prompt("")
	cancel()
	// give tty1 back
	svc, err := machine.Getty(1)
	if err == nil {
		svc.Start()
	}

	return nil
}

func runInstall(options map[string]string) error {
	f, _ := ioutil.TempFile("", "xxxx")

	device, ok := options["device"]
	if !ok {
		fmt.Println("device must be specified among options")
		os.Exit(1)
	}

	cloudInit, ok := options["cc"]
	if !ok {
		fmt.Println("cloudInit must be specified among options")
		os.Exit(1)
	}

	c := &config.Config{}
	yaml.Unmarshal([]byte(cloudInit), c)

	_, reboot := options["reboot"]
	_, poweroff := options["poweroff"]

	ioutil.WriteFile(f.Name(), []byte(cloudInit), os.ModePerm)
	args := []string{"install"}
	args = append(args, optsToArgs(options)...)
	args = append(args, "-c", f.Name(), fmt.Sprintf("%s", device))

	cmd := exec.Command("elemental", args...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if reboot || c.C3OS != nil && c.C3OS.Reboot {
		utils.Reboot()
	}

	if poweroff || c.C3OS != nil && c.C3OS.Poweroff {
		utils.PowerOFF()
	}
	return nil
}
