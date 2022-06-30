package mos_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/c3os-io/c3os/tests/machine"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var _ = Describe("c3os decentralized k8s test", Label("decentralized-k8s"), func() {
	BeforeEach(func() {
		machine.EventuallyConnects()
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			gatherLogs()
		}
	})

	Context("live cd", func() {
		It("has default service active", func() {
			if os.Getenv("FLAVOR") == "alpine" {
				out, _ := machine.SSHCommand("sudo rc-status")
				Expect(out).Should(ContainSubstring("c3os"))
				Expect(out).Should(ContainSubstring("c3os-agent"))
			} else {
				// Eventually(func() string {
				// 	out, _ := machine.SSHCommand("sudo systemctl status c3os-agent")
				// 	return out
				// }, 30*time.Second, 10*time.Second).Should(ContainSubstring("no network token"))

				out, _ := machine.SSHCommand("sudo systemctl status c3os")
				Expect(out).Should(ContainSubstring("loaded (/etc/systemd/system/c3os.service; enabled; vendor preset: disabled)"))
			}
		})
	})

	Context("install", func() {
		It("to disk with custom config", func() {
			err := machine.SendFile(os.Getenv("CLOUD_INIT"), "/tmp/config.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())

			out, _ := machine.SSHCommand("sudo elemental install --cloud-init /tmp/config.yaml /dev/sda")
			Expect(out).Should(ContainSubstring("Running after-install hook"))
			fmt.Println(out)
			machine.SSHCommand("sudo sync")
			machine.DetachCD()
			machine.Restart()
		})
	})

	Context("first-boot", func() {

		It("has default services on", func() {
			if os.Getenv("FLAVOR") == "alpine" {
				out, _ := machine.SSHCommand("sudo rc-status")
				Expect(out).Should(ContainSubstring("c3os"))
				Expect(out).Should(ContainSubstring("c3os-agent"))
			} else {
				// Eventually(func() string {
				// 	out, _ := machine.SSHCommand("sudo systemctl status c3os-agent")
				// 	return out
				// }, 30*time.Second, 10*time.Second).Should(ContainSubstring("no network token"))

				out, _ := machine.SSHCommand("sudo systemctl status c3os-agent")
				Expect(out).Should(ContainSubstring("loaded (/etc/systemd/system/c3os-agent.service; enabled; vendor preset: disabled)"))

				out, _ = machine.SSHCommand("sudo systemctl status systemd-timesyncd")
				Expect(out).Should(ContainSubstring("loaded (/usr/lib/systemd/system/systemd-timesyncd.service; enabled; vendor preset: disabled)"))
			}
		})

		It("has correct grub menu entries", func() {
			if os.Getenv("FLAVOR") == "alpine" {
				Skip("not working on alpine yet")
			}

			By("checking entries", func() {
				state, _ := machine.SSHCommand("sudo blkid -L COS_STATE")
				state = strings.TrimSpace(state)
				out, _ := machine.SSHCommand("sudo blkid")
				fmt.Println(out)
				out, _ = machine.SSHCommand("sudo mkdir -p /tmp/mnt/STATE")
				fmt.Println(out)
				out, _ = machine.SSHCommand("sudo mount " + state + " /tmp/mnt/STATE")
				fmt.Println(out)
				out, _ = machine.SSHCommand("sudo cat /tmp/mnt/STATE/grubmenu")
				Expect(out).Should(ContainSubstring("c3os remote recovery"))

				grub, _ := machine.SSHCommand("sudo cat /tmp/mnt/STATE/grub_oem_env")
				Expect(grub).Should(ContainSubstring("default_menu_entry=c3os"))

				machine.SSHCommand("sudo umount /tmp/mnt/STATE")
			})
		})

		It("has default image sizes", func() {
			for _, p := range []string{"active.img", "passive.img"} {
				out, _ := machine.SSHCommand(`sudo stat -c "%s" /run/initramfs/cos-state/cOS/` + p)
				Expect(out).Should(ContainSubstring("2097152000"))
			}
		})

		It("configure k3s", func() {
			_, err := machine.SSHCommand("cat /run/cos/live_mode")
			Expect(err).To(HaveOccurred())
			if os.Getenv("FLAVOR") == "alpine" {
				Eventually(func() string {
					out, _ := machine.SSHCommand("sudo cat /var/log/c3os-agent.log")
					fmt.Println(out)
					return out
				}, 20*time.Minute, 1*time.Second).Should(
					Or(
						ContainSubstring("Configuring k3s-agent"),
						ContainSubstring("Configuring k3s"),
					))
			} else {
				Eventually(func() string {
					out, _ := machine.SSHCommand("sudo systemctl status c3os-agent")
					return out
				}, 30*time.Minute, 1*time.Second).Should(
					Or(
						ContainSubstring("Configuring k3s-agent"),
						ContainSubstring("Configuring k3s"),
					))
			}
		})

		PIt("configure edgevpn", func() {
			Eventually(func() string {
				out, _ := machine.SSHCommand("sudo cat /etc/systemd/system.conf.d/edgevpn-c3os.env")
				return out
			}, 1*time.Minute, 1*time.Second).Should(
				And(
					ContainSubstring("EDGEVPNLOGLEVEL=\"debug\""),
				))
		})

		It("propagate kubeconfig", func() {
			Eventually(func() string {
				out, _ := machine.SSHCommand("c3os-agent get-kubeconfig")
				return out
			}, 900*time.Second, 10*time.Second).Should(ContainSubstring("https:"))

			Eventually(func() string {
				machine.SSHCommand("c3os get-kubeconfig > kubeconfig")
				out, _ := machine.SSHCommand("KUBECONFIG=kubeconfig kubectl get nodes -o wide")
				return out
			}, 900*time.Second, 10*time.Second).Should(ContainSubstring("Ready"))
		})

		It("has roles", func() {
			uuid, _ := machine.SSHCommand("c3os-agent uuid")
			Expect(uuid).ToNot(Equal(""))
			Eventually(func() string {
				out, _ := machine.SSHCommand("c3os-agent role list")
				return out
			}, 900*time.Second, 10*time.Second).Should(And(
				ContainSubstring(uuid),
				ContainSubstring("worker"),
				ContainSubstring("master"),
				HaveMinMaxRole("master", 1, 1),
				HaveMinMaxRole("worker", 1, 1),
			))
		})

		It("has machines with different IPs", func() {
			Eventually(func() string {
				out, _ := machine.SSHCommand(`curl http://localhost:8080/api/machines`)
				return out
			}, 900*time.Second, 10*time.Second).Should(And(
				ContainSubstring("10.1.0.1"),
				ContainSubstring("10.1.0.2"),
			))
		})

		It("can propagate dns and it is functional", func() {
			if os.Getenv("FLAVOR") == "alpine" {
				Skip("DNS not working on alpine yet")
			}
			Eventually(func() string {
				machine.SSHCommand(`curl -X POST http://localhost:8080/api/dns --header "Content-Type: application/json" -d '{ "Regex": "foo.bar", "Records": { "A": "2.2.2.2" } }'`)
				out, _ := machine.SSHCommand("ping -c 1 foo.bar")
				return out
			}, 900*time.Second, 10*time.Second).Should(And(
				ContainSubstring("2.2.2.2"),
			))
			Eventually(func() string {
				out, _ := machine.SSHCommand("ping -c 1 google.com")
				return out
			}, 900*time.Second, 10*time.Second).Should(And(
				ContainSubstring("64 bytes from"),
			))
		})

		It("upgrades to a specific version", func() {
			version, _ := machine.SSHCommand("source /etc/os-release; echo $VERSION")

			out, _ := machine.SSHCommand("sudo c3os-agent upgrade v1.21.4-32")
			Expect(out).To(ContainSubstring("Upgrade completed"))

			machine.SSHCommand("sudo sync")
			machine.Restart()

			machine.EventuallyConnects(700)

			version2, _ := machine.SSHCommand("source /etc/os-release; echo $VERSION")
			Expect(version).ToNot(Equal(version2))
		})
	})
})

func HaveMinMaxRole(name string, min, max int) types.GomegaMatcher {
	return WithTransform(
		func(actual interface{}) (int, error) {
			switch s := actual.(type) {
			case string:
				return strings.Count(s, name), nil
			default:
				return 0, fmt.Errorf("HaveRoles expects a string, but got %T", actual)
			}
		}, SatisfyAll(
			BeNumerically(">=", min),
			BeNumerically("<=", max)))
}
