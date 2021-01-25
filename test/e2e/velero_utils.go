package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	veleroClient "github.com/vmware-tanzu/velero/pkg/client"
	cliinstall "github.com/vmware-tanzu/velero/pkg/cmd/cli/install"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/flag"
	"github.com/vmware-tanzu/velero/pkg/install"
)

func getProviderPlugins(providerName string) []string {
	// TODO: make plugin images configurable
	switch providerName {
	case "aws":
		return []string{"velero/velero-plugin-for-aws:v1.1.0"}
	case "azure":
		return []string{"velero/velero-plugin-for-microsoft-azure:v1.1.1"}
	case "vsphere":
		return []string{"velero/velero-plugin-for-aws:v1.1.0", "velero/velero-plugin-for-vsphere:v1.0.2"}
	default:
		return []string{""}
	}
}

// GetProviderVeleroInstallOptions returns Velero InstallOptions for the provider.
func GetProviderVeleroInstallOptions(pluginProvider, credentialsFile, objectStoreBucket, objectStorePrefix string,
	bslConfig, vslConfig string,
	plugins []string,
) (*cliinstall.InstallOptions, error) {

	if credentialsFile == "" {
		return nil, errors.Errorf("No credentials were supplied to use for E2E tests")
	}

	realPath, err := filepath.Abs(credentialsFile)
	if err != nil {
		return nil, err
	}

	io := cliinstall.NewInstallOptions()
	// always wait for velero and restic pods to be running.
	io.Wait = true
	io.ProviderName = pluginProvider
	io.SecretFile = credentialsFile

	io.BucketName = objectStoreBucket
	io.Prefix = objectStorePrefix
	io.BackupStorageConfig = flag.NewMap()
	io.BackupStorageConfig.Set(bslConfig)

	io.VolumeSnapshotConfig = flag.NewMap()
	io.VolumeSnapshotConfig.Set(vslConfig)

	io.SecretFile = realPath
	io.Plugins = flag.NewStringArray(plugins...)
	return io, nil
}

// InstallVeleroServer installs velero in the cluster.
func InstallVeleroServer(io *cliinstall.InstallOptions) error {
	config, err := veleroClient.LoadConfig()
	if err != nil {
		return err
	}

	vo, err := io.AsVeleroOptions()
	if err != nil {
		return errors.Wrap(err, "Failed to translate InstallOptions to VeleroOptions for Velero")
	}

	f := veleroClient.NewFactory("e2e", config)
	resources, err := install.AllResources(vo)
	if err != nil {
		return errors.Wrap(err, "Failed to install Velero in the cluster")
	}

	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return err
	}
	factory := veleroClient.NewDynamicFactory(dynamicClient)
	errorMsg := "\n\nError installing Velero. Use `kubectl logs deploy/velero -n velero` to check the deploy logs"
	err = install.Install(factory, resources, os.Stdout)
	if err != nil {
		return errors.Wrap(err, errorMsg)
	}

	fmt.Println("Waiting for Velero deployment to be ready.")
	if _, err = install.DeploymentIsReady(factory, io.Namespace); err != nil {
		return errors.Wrap(err, errorMsg)
	}

	// restic enabled by default
	fmt.Println("Waiting for Velero restic daemonset to be ready.")
	if _, err = install.DaemonSetIsReady(factory, io.Namespace); err != nil {
		return errors.Wrap(err, errorMsg)
	}

	return nil
}

// CheckBackupPhase uses veleroCLI to inspect the phase of a Velero backup.
func CheckBackupPhase(ctx context.Context, veleroCLI string, veleroNamespace string, backupName string,
	expectedPhase velerov1api.BackupPhase) error {
	checkCMD := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "backup", "get", "-o", "json",
		backupName)

	fmt.Printf("get backup cmd =%v\n", checkCMD)
	stdoutPipe, err := checkCMD.StdoutPipe()
	if err != nil {
		return err
	}

	jsonBuf := make([]byte, 16*1024) // If the YAML is bigger than 16K, there's probably something bad happening

	err = checkCMD.Start()
	if err != nil {
		return err
	}

	bytesRead, err := io.ReadFull(stdoutPipe, jsonBuf)

	if err != nil && err != io.ErrUnexpectedEOF {
		return err
	}
	if bytesRead == len(jsonBuf) {
		return errors.New("yaml returned bigger than max allowed")
	}

	jsonBuf = jsonBuf[0:bytesRead]
	err = checkCMD.Wait()
	if err != nil {
		return err
	}
	backup := velerov1api.Backup{}
	err = json.Unmarshal(jsonBuf, &backup)
	if err != nil {
		return err
	}
	if backup.Status.Phase != expectedPhase {
		return errors.Errorf("Unexpected backup phase got %s, expecting %s", backup.Status.Phase, expectedPhase)
	}
	return nil
}

// CheckRestorePhase uses veleroCLI to inspect the phase of a Velero restore.
func CheckRestorePhase(ctx context.Context, veleroCLI string, veleroNamespace string, restoreName string,
	expectedPhase velerov1api.RestorePhase) error {
	checkCMD := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "restore", "get", "-o", "json",
		restoreName)

	fmt.Printf("get restore cmd =%v\n", checkCMD)
	stdoutPipe, err := checkCMD.StdoutPipe()
	if err != nil {
		return err
	}

	jsonBuf := make([]byte, 16*1024) // If the YAML is bigger than 16K, there's probably something bad happening

	err = checkCMD.Start()
	if err != nil {
		return err
	}

	bytesRead, err := io.ReadFull(stdoutPipe, jsonBuf)

	if err != nil && err != io.ErrUnexpectedEOF {
		return err
	}
	if bytesRead == len(jsonBuf) {
		return errors.New("yaml returned bigger than max allowed")
	}

	jsonBuf = jsonBuf[0:bytesRead]
	err = checkCMD.Wait()
	if err != nil {
		return err
	}
	restore := velerov1api.Restore{}
	err = json.Unmarshal(jsonBuf, &restore)
	if err != nil {
		return err
	}
	if restore.Status.Phase != expectedPhase {
		return errors.Errorf("Unexpected restore phase got %s, expecting %s", restore.Status.Phase, expectedPhase)
	}
	return nil
}

// VeleroBackupNamespace uses the veleroCLI to backup a namespace.
func VeleroBackupNamespace(ctx context.Context, veleroCLI string, veleroNamespace string, backupName string, namespace string) error {
	backupCmd := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "create", "backup", backupName,
		"--include-namespaces", namespace,
		"--default-volumes-to-restic", "--wait")
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	fmt.Printf("backup cmd =%v\n", backupCmd)
	err := backupCmd.Run()
	if err != nil {
		return err
	}
	err = CheckBackupPhase(ctx, veleroCLI, veleroNamespace, backupName, velerov1api.BackupPhaseCompleted)

	return err
}

// VeleroBackupExcludeNamespaces uses the veleroCLI to backup a namespace.
func VeleroBackupExcludeNamespaces(ctx context.Context, veleroCLI string, veleroNamespace string, backupName string, excludeNamespaces []string) error {
	namespaces := ""
	for nsNum, curNamespace := range excludeNamespaces {
		if nsNum > 0 {
			namespaces = namespaces + ","
		}
		namespaces = namespaces + curNamespace
	}
	backupCmd := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "create", "backup", backupName,
		"--exclude-namespaces", namespaces,
		"--default-volumes-to-restic", "--wait")

	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	fmt.Printf("backup cmd =%v\n", backupCmd)
	err := backupCmd.Run()
	if err != nil {
		return err
	}
	err = CheckBackupPhase(ctx, veleroCLI, veleroNamespace, backupName, velerov1api.BackupPhaseCompleted)

	return err
}

// VeleroRestore uses the veleroCLI to restore from a Velero backup.
func VeleroRestore(ctx context.Context, veleroCLI string, veleroNamespace string, restoreName string, backupName string) error {
	restoreCmd := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "create", "restore", restoreName,
		"--from-backup", backupName, "--wait")

	restoreCmd.Stdout = os.Stdout
	restoreCmd.Stderr = os.Stderr
	fmt.Printf("restore cmd =%v\n", restoreCmd)
	err := restoreCmd.Run()
	if err != nil {
		return err
	}
	return CheckRestorePhase(ctx, veleroCLI, veleroNamespace, restoreName, velerov1api.RestorePhaseCompleted)
}

func VeleroInstall(ctx context.Context, veleroNamespace string, cloudProvider string, objectStoreProvider string, useVolumeSnapshots bool,
	cloudCredentialsFile string, bslBucket string, bslPrefix string, bslConfig string, vslConfig string) error {

	if cloudProvider != "kind" {
		if objectStoreProvider != "" {
			return errors.New("For cloud platforms, object store plugin cannot be overriden") // Can't set an object store provider that is different than your cloud
		}
		objectStoreProvider = cloudProvider
	} else {
		if objectStoreProvider == "" {
			return errors.New("No object store provider specified - must be specified when using kind as the cloud provider") // Gotta have an object store provider
		}
	}
	err := EnsureClusterExists(ctx)
	if err != nil {
		return errors.WithMessage(err, "Failed to ensure kubernetes cluster exists")
	}
	veleroInstallOptions, err := GetProviderVeleroInstallOptions(objectStoreProvider, cloudCredentialsFile, bslBucket, bslPrefix, bslConfig, vslConfig, getProviderPlugins(objectStoreProvider))
	if err != nil {
		return errors.WithMessagef(err, "Failed to get Velero InstallOptions for plugin provider %s", objectStoreProvider)
	}
	veleroInstallOptions.UseRestic = !useVolumeSnapshots
	veleroInstallOptions.Namespace = veleroNamespace
	err = InstallVeleroServer(veleroInstallOptions)
	if err != nil {
		return errors.WithMessagef(err, "Failed to install Velero in cluster")
	}
	return nil
}

func VeleroUninstall(ctx context.Context, client *kubernetes.Clientset, veleroNamespace string) error {
	return client.CoreV1().Namespaces().Delete(ctx, veleroNamespace, metav1.DeleteOptions{})
}

func VeleroBackupLogs(ctx context.Context, veleroCLI string, veleroNamespace string, backupName string) error {
	describeCmd := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "backup", "describe", backupName)
	describeCmd.Stdout = os.Stdout
	describeCmd.Stderr = os.Stderr
	err := describeCmd.Run()
	if err != nil {
		return err
	}
	logCmd := exec.CommandContext(ctx, veleroCLI, "--namespace", veleroNamespace, "backup", "logs", backupName)
	logCmd.Stdout = os.Stdout
	logCmd.Stderr = os.Stderr
	err = logCmd.Run()
	if err != nil {
		return err
	}
	return nil
}
