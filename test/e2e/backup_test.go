package e2e

import (
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
)

var (
	uuidgen uuid.UUID
)

// Test backup and restore of Kibishi using restic
var _ = Describe("[Restic] Velero tests on cluster using the plugin provider for object storage and Restic for volume backups", func() {
	var (
		client      *kubernetes.Clientset
		backupName  string
		restoreName string
	)

	BeforeEach(func() {
		var err error
		client, err = GetClusterClient()
		Expect(err).To(Succeed(), "Failed to instantiate cluster client")
	})

	Context("When kibishii is the sample workload", func() {
		It("should be successfully backed up and restored", func() {
			backupName = "backup-" + uuidgen.String()
			restoreName = "restore-" + uuidgen.String()
			// Even though we are using Velero's CloudProvider plugin for object storage, the kubernetes cluster is running on
			// KinD. So use the kind installation for Kibishii.
			Expect(RunKibishiiTests(client, cloudProvider, veleroCLI, veleroNamespace, backupName, restoreName)).To(Succeed(),
				"Failed to successfully backup and restore Kibishii namespace")
		})
	})
})
