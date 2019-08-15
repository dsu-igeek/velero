# Run Velero on vSphere

To set up Velero on vSphere, you:

* Download an official release of Velero
* Create an object store destination
* Install the server
* Designate Pods with the restic annotation

If you do not have the `aws` CLI locally installed, follow the [user guide][5] to set it up.

## Download Velero

1. Download the [latest official release's](https://github.com/heptio/velero/releases) tarball for your client platform.

    _We strongly recommend that you use an [official release](https://github.com/heptio/velero/releases) of
Velero. The tarballs for each release contain the `velero` command-line client. The code in the master branch
of the Velero repository is under active development and is not guaranteed to be stable!_

2. Extract the tarball:

    ```
    tar -xvf <RELEASE-TARBALL-NAME>.tar.gz -C /dir/to/extract/to
    ```

    We'll refer to the directory you extracted to as the "Velero directory" in subsequent steps.

3. Move the `velero` binary from the Velero directory to somewhere in your PATH.

## Create object storage destination

Velero requires an object storage bucket to store backups in, preferrably unique to a single Kubernetes cluster (see the [FAQ][20] for more details). 

You may use any of the supported object stores with vSphere.  Please refer to the appropriate Velero guide for the object storage you would 
like to use.

## Install and start Velero

Install Velero, including all prerequisites, into the cluster and start the deployment. This will create a namespace called `velero`,
 and place a deployment named `velero` in it.
At present, vSphere only supports backup of PVs using Restic.  Please be sure to specify the --use-restic flag when installing velero.
```bash
velero install \
    --provider aws \
    --bucket $BUCKET \
    --secret-file ./credentials-velero \
    --backup-location-config region=$REGION \
    --use-restic \
    --snapshot-location-config region=$REGION
```

You can specify `--wait` to wait for the deployment to be ready.

(Optional) Specify [additional configurable parameters][21] for the `--backup-location-config` flag.

(Optional) Specify [additional configurable parameters][6] for the `--snapshot-location-config` flag.

(Optional) Specify [CPU and memory resource requests and limits][22] for the Velero/restic pods.

For more complex installation needs, use either the Helm chart, or add `--dry-run -o yaml` options for generating the YAML representation for the installation.

## Designate pods with Restic annotation

Pods with Persistent Volumes can be backed up and restore with the help of Restic. First annotate the PVs so that Velero knows to back them up. Annotate each volume associated to the pod.

```bash
kubectl -n kibishii annotate pod/<pod-name> backup.velero.io/backup-volumes=<volume-mount>
```bash

Create backup with velero.
```bash
velero backup create <backup-name> --include-namespaces <namespace>
```bash

Verify the velero backup logs to if the backup is succuessful which includes PVs.

Now restore the backed up files from velero and verify the data on the volumes mounted in the Pods.

```bash
velero restore create --from-backup <backup-name>
```bash

With this, Backup and Restore of Pods is successful on vSphere.



[0]: namespace.md
[5]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-welcome.html
[6]: api-types/volumesnapshotlocation.md#aws
[14]: http://docs.aws.amazon.com/IAM/latest/UserGuide/introduction.html
[20]: faq.md
[21]: api-types/backupstoragelocation.md#aws
[22]: install-overview.md#velero-resource-requirements
