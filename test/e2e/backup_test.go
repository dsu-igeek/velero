package e2e

import (
	"flag"
	"fmt"
	"os/exec"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

var (
	veleroCLI  string
	namespace  string
	backupName string
)

func init() {
	flag.StringVar(&veleroCLI, "velerocli", "velero", "path to the velero application to use")
	flag.StringVar(&namespace, "kibishiins", "kibishii", "namespace to use for Kibishii distributed data generator")
}

var _ = Describe("Backup", func() {

	BeforeEach(func() {
		flag.Parse()
	})
	Describe("backing up and restoring namespace with data", func() {
		Context("when the backup is successful", func() {
			It("generates data, backups up the namespace, deletes the namespace, restores the namespace and verifies data", func() {
				backupUUID, err := uuid.NewRandom()
				Expect(err).NotTo(HaveOccurred())
				backupName = backupUUID.String()
				println("backupName = " + backupName)
				println("creating namespace " + namespace)
				timeoutCTX, _ := context.WithTimeout(context.Background(), time.Minute)
				err = CreateNamespace(timeoutCTX, namespace)
				Expect(err).NotTo(HaveOccurred())

				println("installing kibishii in namespace " + namespace)
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute)
				err = InstallKibishii(timeoutCTX, namespace)
				Expect(err).NotTo(HaveOccurred())

				println("running kibishii generate")
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute*60)

				err = GenerateData(timeoutCTX, namespace, 2, 10, 10, 1024, 1024, 0, 2)
				Expect(err).NotTo(HaveOccurred())

				println("executing backup")
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute*30)

				backupCmd := exec.CommandContext(timeoutCTX, veleroCLI, "create", "backup", backupName, "--include-namespaces", namespace,
					"--default-volumes-to-restic", "--wait")
				fmt.Printf("backup cmd =%v\n", backupCmd)
				err = backupCmd.Run()
				Expect(err).NotTo(HaveOccurred())
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute)
				err = CheckBackupPhase(timeoutCTX, veleroCLI, backupName, velerov1.BackupPhaseCompleted)
				//err := CheckBackupPhase(timeoutCTX, veleroCLI, "01e4f4be-9e24-4a83-8a88-f364b0d929d8", velerov1.BackupPhaseCompleted)

				Expect(err).NotTo(HaveOccurred())

				println("removing namespace " + namespace)
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute)
				err = RemoveNamespace(timeoutCTX, namespace)
				Expect(err).NotTo(HaveOccurred())

				println("restoring namespace")
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute*30)
				restoreCmd := exec.CommandContext(timeoutCTX, veleroCLI, "create", "restore", "--from-backup", backupName, "--wait")
				fmt.Printf("restore cmd =%v\n", restoreCmd)
				err = restoreCmd.Run()
				Expect(err).NotTo(HaveOccurred())
				println("Checking that namespace is present")
				// TODO - check that namespace exists
				println("running kibishii verify")
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute*60)

				err = VerifyData(timeoutCTX, namespace, 2, 10, 10, 1024, 1024, 0, 2)
				Expect(err).NotTo(HaveOccurred())

				println("removing namespace " + namespace)
				timeoutCTX, _ = context.WithTimeout(context.Background(), time.Minute)
				err = RemoveNamespace(timeoutCTX, namespace)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
