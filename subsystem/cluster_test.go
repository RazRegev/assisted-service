package subsystem

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/alecthomas/units"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	. "github.com/onsi/gomega"
	"github.com/openshift/assisted-service/client/installer"
	"github.com/openshift/assisted-service/models"
)

// #nosec
const (
	clusterInsufficientStateInfo                = "Cluster is not ready for install"
	clusterReadyStateInfo                       = "Cluster ready to be installed"
	sshPublicKey                                = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC50TuHS7aYci+U+5PLe/aW/I6maBi9PBDucLje6C6gtArfjy7udWA1DCSIQd+DkHhi57/s+PmvEjzfAfzqo+L+/8/O2l2seR1pPhHDxMR/rSyo/6rZP6KIL8HwFqXHHpDUM4tLXdgwKAe1LxBevLt/yNl8kOiHJESUSl+2QSf8z4SIbo/frDD8OwOvtfKBEG4WCb8zEsEuIPNF/Vo/UxPtS9pPTecEsWKDHR67yFjjamoyLvAzMAJotYgyMoxm8PTyCgEzHk3s3S4iO956d6KVOEJVXnTVhAxrtLuubjskd7N4hVN7h2s4Z584wYLKYhrIBL0EViihOMzY4mH3YE4KZusfIx6oMcggKX9b3NHm0la7cj2zg0r6zjUn6ZCP4gXM99e5q4auc0OEfoSfQwofGi3WmxkG3tEozCB8Zz0wGbi2CzR8zlcF+BNV5I2LESlLzjPY5B4dvv5zjxsYoz94p3rUhKnnPM2zTx1kkilDK5C5fC1k9l/I/r5Qk4ebLQU= oscohen@localhost.localdomain"
	pullSecret                                  = "{\"auths\":{\"cloud.openshift.com\":{\"auth\":\"dXNlcjpwYXNzd29yZAo=\",\"email\":\"r@r.com\"}}}"
	IgnoreStateInfo                             = "IgnoreStateInfo"
	clusterCanceledInfo                         = "Canceled cluster installation"
	clusterErrorInfo                            = "cluster has hosts in error"
	clusterResetStateInfo                       = "cluster was reset by user"
	clusterPendingForInputStateInfo             = "User input required"
	clusterFinalizingStateInfo                  = "Finalizing cluster installation"
	clusterInstallingPendingUserActionStateInfo = "Cluster has hosts with wrong boot order"
	clusterInstallingStateInfo                  = "Installation in progress"
)

const (
	validDiskSize     = int64(128849018880)
	minSuccessesInRow = 2
)

var (
	validWorkerHwInfo = &models.Inventory{
		CPU:    &models.CPU{Count: 2},
		Memory: &models.Memory{PhysicalBytes: int64(8 * units.GiB)},
		Disks: []*models.Disk{
			{DriveType: "SSD", Name: "loop0", SizeBytes: validDiskSize},
			{DriveType: "HDD", Name: "sdb", SizeBytes: validDiskSize}},
		Interfaces: []*models.Interface{
			{IPV4Addresses: []string{"1.2.3.4/24"}},
		},
		SystemVendor: &models.SystemVendor{Manufacturer: "manu", ProductName: "prod", SerialNumber: "3534"},
		Timestamp:    1601853088,
	}
	validMasterHwInfo = &models.Inventory{
		CPU:    &models.CPU{Count: 16},
		Memory: &models.Memory{PhysicalBytes: int64(32 * units.GiB)},
		Disks: []*models.Disk{
			{DriveType: "SSD", Name: "loop0", SizeBytes: validDiskSize},
			{DriveType: "HDD", Name: "sdb", SizeBytes: validDiskSize}},
		Interfaces: []*models.Interface{
			{IPV4Addresses: []string{"1.2.3.4/24"}},
		},
		SystemVendor: &models.SystemVendor{Manufacturer: "manu", ProductName: "prod", SerialNumber: "3534"},
		Timestamp:    1601853088,
	}
	validHwInfo = &models.Inventory{
		CPU:    &models.CPU{Count: 16},
		Memory: &models.Memory{PhysicalBytes: int64(32 * units.GiB)},
		Disks: []*models.Disk{
			{DriveType: "SSD", Name: "loop0", SizeBytes: validDiskSize},
			{DriveType: "HDD", Name: "sdb", SizeBytes: validDiskSize}},
		Interfaces: []*models.Interface{
			{
				IPV4Addresses: []string{
					"1.2.3.4/24",
				},
			},
		},
		SystemVendor: &models.SystemVendor{Manufacturer: "manu", ProductName: "prod", SerialNumber: "3534"},
		Timestamp:    1601853088,
	}
	validFreeAddresses = models.FreeNetworksAddresses{
		{
			Network: "1.2.3.0/24",
			FreeAddresses: []strfmt.IPv4{
				"1.2.3.8",
				"1.2.3.9",
				"1.2.3.5",
				"1.2.3.6",
			},
		},
	}
	validNtpSources = []*models.NtpSource{
		{SourceName: "clock.dummy.com", SourceState: models.SourceStateSynced},
	}
)

func isClusterInState(ctx context.Context, clusterID strfmt.UUID, state, stateInfo string) (bool, string) {
	rep, err := userBMClient.Installer.GetCluster(ctx, &installer.GetClusterParams{ClusterID: clusterID})
	Expect(err).NotTo(HaveOccurred())
	c := rep.GetPayload()
	if swag.StringValue(c.Status) == state {
		return stateInfo == IgnoreStateInfo || swag.StringValue(c.StatusInfo) == stateInfo, swag.StringValue(c.Status)
	}
	Expect(swag.StringValue(c.Status)).NotTo(Equal("error"))

	return false, swag.StringValue(c.Status)
}

func waitForClusterState(ctx context.Context, clusterID strfmt.UUID, state string, timeout time.Duration, stateInfo string) {
	log.Infof("Waiting for cluster %s status %s", clusterID, state)
	var lastState string = ""
	var success bool

	for start, successInRow := time.Now(), 0; time.Since(start) < timeout; {
		success, lastState = isClusterInState(ctx, clusterID, state, stateInfo)

		if success {
			successInRow++
		} else {
			successInRow = 0
		}

		// Wait for cluster state to be consistent
		if successInRow >= minSuccessesInRow {
			log.Infof("cluster %s has status %s", clusterID, state)
			return
		}

		time.Sleep(time.Second)
	}

	Expect(lastState).Should(Equal(state), fmt.Sprintf("Cluster %s wasn't in state %s for %d times in a row.",
		clusterID, state, minSuccessesInRow))
}

func isHostInState(ctx context.Context, clusterID strfmt.UUID, hostID strfmt.UUID, state string) (bool, string) {
	rep, err := userBMClient.Installer.GetHost(ctx, &installer.GetHostParams{ClusterID: clusterID, HostID: hostID})
	Expect(err).NotTo(HaveOccurred())
	h := rep.GetPayload()
	return swag.StringValue(h.Status) == state, swag.StringValue(h.Status)
}

func waitForHostState(ctx context.Context, clusterID strfmt.UUID, hostID strfmt.UUID, state string, timeout time.Duration) {
	log.Infof("Waiting for host %s state %s", hostID, state)
	var lastState string = ""
	var success bool

	for start, successInRow := time.Now(), 0; time.Since(start) < timeout; {
		success, lastState = isHostInState(ctx, clusterID, hostID, state)

		if success {
			successInRow++
		} else {
			successInRow = 0
		}

		// Wait for host state to be consistent
		if successInRow >= minSuccessesInRow {
			log.Infof("host %s has status %s", clusterID, state)
			return
		}

		time.Sleep(time.Second)
	}

	Expect(lastState).Should(Equal(state), fmt.Sprintf("Host %s in Cluster %s wasn't in state %s for %d times in a row.",
		hostID, clusterID, state, minSuccessesInRow))
}

func installCluster(clusterID strfmt.UUID) *models.Cluster {
	ctx := context.Background()
	reply, err := userBMClient.Installer.InstallCluster(ctx, &installer.InstallClusterParams{ClusterID: clusterID})
	Expect(err).NotTo(HaveOccurred())
	c := reply.GetPayload()
	Expect(*c.Status).Should(Equal(models.ClusterStatusPreparingForInstallation))

	waitForClusterState(ctx, clusterID, models.ClusterStatusInstalling,
		180*time.Second, "Installation in progress")

	rep, err := userBMClient.Installer.GetCluster(ctx, &installer.GetClusterParams{ClusterID: clusterID})
	Expect(err).NotTo(HaveOccurred())
	c = rep.GetPayload()
	Expect(c).NotTo(BeNil())

	return c
}

func installClusterAndComplete(clusterID strfmt.UUID) {
	c := installCluster(clusterID)
	Expect(len(c.Hosts)).Should(Equal(5))
	for _, host := range c.Hosts {
		Expect(swag.StringValue(host.Status)).Should(Equal("installing"))
	}

	for _, host := range c.Hosts {
		updateProgress(*host.ID, clusterID, models.HostStageDone)
	}

	waitForClusterState(context.Background(), clusterID, models.ClusterStatusFinalizing, defaultWaitForClusterStateTimeout, clusterFinalizingStateInfo)

	success := true
	_, err := agentBMClient.Installer.CompleteInstallation(context.Background(),
		&installer.CompleteInstallationParams{ClusterID: clusterID, CompletionParams: &models.CompletionParams{IsSuccess: &success, ErrorInfo: ""}})
	Expect(err).NotTo(HaveOccurred())

}

func FailCluster(ctx context.Context, clusterID strfmt.UUID) strfmt.UUID {
	c := installCluster(clusterID)
	var masterHostID strfmt.UUID = *getClusterMasters(c)[0].ID

	installStep := models.HostStageFailed
	installInfo := "because some error"

	updateProgressWithInfo(masterHostID, clusterID, installStep, installInfo)
	masterHost := getHost(clusterID, masterHostID)
	Expect(*masterHost.Status).Should(Equal("error"))
	Expect(*masterHost.StatusInfo).Should(Equal(fmt.Sprintf("%s - %s", installStep, installInfo)))
	return masterHostID
}

func registerHostsAndSetRoles(clusterID strfmt.UUID, numHosts int) []*models.Host {
	ctx := context.Background()
	hosts := make([]*models.Host, 0)

	generateFAPostStepReply := func(h *models.Host, freeAddresses models.FreeNetworksAddresses) {
		fa, err := json.Marshal(&freeAddresses)
		Expect(err).NotTo(HaveOccurred())
		_, err = agentBMClient.Installer.PostStepReply(ctx, &installer.PostStepReplyParams{
			ClusterID: h.ClusterID,
			HostID:    *h.ID,
			Reply: &models.StepReply{
				ExitCode: 0,
				Output:   string(fa),
				StepID:   string(models.StepTypeFreeNetworkAddresses),
				StepType: models.StepTypeFreeNetworkAddresses,
			},
		})
		Expect(err).ShouldNot(HaveOccurred())
	}
	for i := 0; i < numHosts; i++ {
		hostname := fmt.Sprintf("h%d", i)
		host := &registerHost(clusterID).Host
		generateHWPostStepReply(ctx, host, validHwInfo, hostname)
		generateFAPostStepReply(host, validFreeAddresses)
		generateNTPPostStepReply(ctx, host, validNtpSources)
		var role models.HostRoleUpdateParams
		if i < 3 {
			role = models.HostRoleUpdateParamsMaster
		} else {
			role = models.HostRoleUpdateParamsWorker
		}
		_, err := userBMClient.Installer.UpdateCluster(ctx, &installer.UpdateClusterParams{
			ClusterUpdateParams: &models.ClusterUpdateParams{HostsRoles: []*models.ClusterUpdateParamsHostsRolesItems0{
				{ID: *host.ID, Role: role},
			}},
			ClusterID: clusterID,
		})
		Expect(err).NotTo(HaveOccurred())
		hosts = append(hosts, host)
	}
	generateFullMeshConnectivity(ctx, "1.2.3.10", hosts...)
	apiVip := ""
	ingressVip := ""
	_, err := userBMClient.Installer.UpdateCluster(ctx, &installer.UpdateClusterParams{
		ClusterUpdateParams: &models.ClusterUpdateParams{
			VipDhcpAllocation: swag.Bool(false),
			APIVip:            &apiVip,
			IngressVip:        &ingressVip,
		},
		ClusterID: clusterID,
	})
	Expect(err).NotTo(HaveOccurred())
	apiVip = "1.2.3.8"
	ingressVip = "1.2.3.9"
	_, err = userBMClient.Installer.UpdateCluster(ctx, &installer.UpdateClusterParams{
		ClusterUpdateParams: &models.ClusterUpdateParams{
			APIVip:     &apiVip,
			IngressVip: &ingressVip,
		},
		ClusterID: clusterID,
	})

	Expect(err).NotTo(HaveOccurred())
	waitForClusterState(ctx, clusterID, models.ClusterStatusReady, 60*time.Second, clusterReadyStateInfo)

	return hosts
}

func getClusterMasters(c *models.Cluster) (masters []*models.Host) {
	for _, host := range c.Hosts {
		if host.Role == models.HostRoleMaster {
			masters = append(masters, host)
		}
	}

	return
}

func generateConnectivityPostStepReply(ctx context.Context, h *models.Host, connectivityReport *models.ConnectivityReport) {
	fa, err := json.Marshal(connectivityReport)
	Expect(err).NotTo(HaveOccurred())
	_, err = agentBMClient.Installer.PostStepReply(ctx, &installer.PostStepReplyParams{
		ClusterID: h.ClusterID,
		HostID:    *h.ID,
		Reply: &models.StepReply{
			ExitCode: 0,
			Output:   string(fa),
			StepID:   string(models.StepTypeConnectivityCheck),
			StepType: models.StepTypeConnectivityCheck,
		},
	})
	Expect(err).ShouldNot(HaveOccurred())
}

func generateFullMeshConnectivity(ctx context.Context, startIPAddress string, hosts ...*models.Host) {

	ip := net.ParseIP(startIPAddress)
	hostToAddr := make(map[strfmt.UUID]string)

	for _, h := range hosts {
		hostToAddr[*h.ID] = ip.String()
		ip[len(ip)-1]++
	}

	var connectivityReport models.ConnectivityReport
	for _, h := range hosts {

		l2Connectivity := make([]*models.L2Connectivity, 0)
		for id, addr := range hostToAddr {

			if id == *h.ID {
				continue
			}

			l2Connectivity = append(l2Connectivity, &models.L2Connectivity{
				RemoteIPAddress: addr,
				Successful:      true,
			})
		}

		connectivityReport.RemoteHosts = append(connectivityReport.RemoteHosts, &models.ConnectivityRemoteHost{
			HostID:         *h.ID,
			L2Connectivity: l2Connectivity,
		})
	}

	for _, h := range hosts {
		generateConnectivityPostStepReply(ctx, h, &connectivityReport)
	}
}

func operatorsFromString(operatorsStr string) models.Operators {

	if operatorsStr != "" {
		var operators models.Operators
		if err := json.Unmarshal([]byte(operatorsStr), &operators); err != nil {
			return nil
		}
		return operators
	}
	return nil
}
