package workflows_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/aws/eks-anywhere/internal/test"
	"github.com/aws/eks-anywhere/pkg/bootstrapper"
	"github.com/aws/eks-anywhere/pkg/cluster"
	writermocks "github.com/aws/eks-anywhere/pkg/filewriter/mocks"
	"github.com/aws/eks-anywhere/pkg/providers"
	providermocks "github.com/aws/eks-anywhere/pkg/providers/mocks"
	"github.com/aws/eks-anywhere/pkg/types"
	"github.com/aws/eks-anywhere/pkg/workflows"
	"github.com/aws/eks-anywhere/pkg/workflows/interfaces/mocks"
)

type createTestSetup struct {
	t                *testing.T
	bootstrapper     *mocks.MockBootstrapper
	clusterManager   *mocks.MockClusterManager
	addonManager     *mocks.MockAddonManager
	provider         *providermocks.MockProvider
	writer           *writermocks.MockFileWriter
	datacenterConfig *providermocks.MockDatacenterConfig
	machineConfigs   []providers.MachineConfig
	workflow         *workflows.Create
	ctx              context.Context
	clusterSpec      *cluster.Spec
	forceCleanup     bool
	bootstrapCluster *types.Cluster
	workloadCluster  *types.Cluster
}

func newCreateTest(t *testing.T) *createTestSetup {
	mockCtrl := gomock.NewController(t)
	bootstrapper := mocks.NewMockBootstrapper(mockCtrl)
	clusterManager := mocks.NewMockClusterManager(mockCtrl)
	addonManager := mocks.NewMockAddonManager(mockCtrl)
	provider := providermocks.NewMockProvider(mockCtrl)
	writer := writermocks.NewMockFileWriter(mockCtrl)
	datacenterConfig := providermocks.NewMockDatacenterConfig(mockCtrl)
	machineConfigs := []providers.MachineConfig{providermocks.NewMockMachineConfig(mockCtrl)}
	workflow := workflows.NewCreate(bootstrapper, provider, clusterManager, addonManager, writer)

	return &createTestSetup{
		t:                t,
		bootstrapper:     bootstrapper,
		clusterManager:   clusterManager,
		addonManager:     addonManager,
		provider:         provider,
		writer:           writer,
		datacenterConfig: datacenterConfig,
		machineConfigs:   machineConfigs,
		workflow:         workflow,
		ctx:              context.Background(),
		clusterSpec:      test.NewClusterSpec(func(s *cluster.Spec) { s.Name = "cluster-name"; s.Annotations = map[string]string{} }),
		bootstrapCluster: &types.Cluster{Name: "bootstrap"},
		workloadCluster:  &types.Cluster{Name: "workload"},
	}
}

func (c *createTestSetup) expectSetup() {
	c.provider.EXPECT().SetupAndValidateCreateCluster(c.ctx, c.clusterSpec)
	c.provider.EXPECT().Name()
	c.addonManager.EXPECT().Validations(c.ctx, c.clusterSpec)
	c.datacenterConfig.EXPECT().Kind().Return("SUP").AnyTimes()
}

func (c *createTestSetup) expectCreateBootstrap() {
	opts := []bootstrapper.BootstrapClusterOption{
		bootstrapper.WithDefaultCNIDisabled(), bootstrapper.WithExtraDockerMounts(),
	}

	gomock.InOrder(
		c.provider.EXPECT().BootstrapClusterOpts().Return(opts, nil),
		// Checking for not nil because in go you can't compare closures
		c.bootstrapper.EXPECT().CreateBootstrapCluster(
			c.ctx, c.clusterSpec, gomock.Not(gomock.Nil()), gomock.Not(gomock.Nil()),
		).Return(c.bootstrapCluster, nil),

		c.clusterManager.EXPECT().InstallCapi(c.ctx, c.clusterSpec, c.bootstrapCluster, c.provider),

		c.provider.EXPECT().BootstrapSetup(c.ctx, c.clusterSpec.Cluster, c.bootstrapCluster),
	)
}

func (c *createTestSetup) expectCreateWorkload() {
	gomock.InOrder(
		c.clusterManager.EXPECT().CreateWorkloadCluster(
			c.ctx, c.bootstrapCluster, c.clusterSpec, c.provider,
		).Return(c.workloadCluster, nil),

		c.clusterManager.EXPECT().InstallNetworking(
			c.ctx, c.workloadCluster, c.clusterSpec,
		),
		c.clusterManager.EXPECT().InstallStorageClass(
			c.ctx, c.workloadCluster, c.provider,
		),
		c.clusterManager.EXPECT().InstallCapi(
			c.ctx, c.clusterSpec, c.workloadCluster, c.provider,
		),
	)
}

func (c *createTestSetup) expectMoveManagement() {
	c.clusterManager.EXPECT().MoveCapi(
		c.ctx, c.bootstrapCluster, c.workloadCluster, gomock.Any(),
	)
}

func (c *createTestSetup) expectInstallEksaComponents() {
	gomock.InOrder(
		c.clusterManager.EXPECT().InstallCustomComponents(
			c.ctx, c.clusterSpec, c.workloadCluster),

		c.provider.EXPECT().DatacenterConfig().Return(c.datacenterConfig),

		c.provider.EXPECT().MachineConfigs().Return(c.machineConfigs),

		c.datacenterConfig.EXPECT().PauseReconcile(),

		c.clusterManager.EXPECT().CreateEKSAResources(
			c.ctx, c.workloadCluster, c.clusterSpec, c.datacenterConfig, c.machineConfigs,
		),

		c.clusterManager.EXPECT().ResumeEKSAControllerReconcile(c.ctx, c.workloadCluster, c.clusterSpec, c.provider),
	)
}

func (c *createTestSetup) expectInstallAddonManager() {
	gomock.InOrder(
		c.provider.EXPECT().DatacenterConfig().Return(c.datacenterConfig),
		c.provider.EXPECT().MachineConfigs().Return(c.machineConfigs),

		c.addonManager.EXPECT().InstallGitOps(
			c.ctx, c.workloadCluster, c.clusterSpec, c.datacenterConfig, c.machineConfigs),
	)
}

func (c *createTestSetup) expectWriteClusterConfig() {
	gomock.InOrder(
		c.provider.EXPECT().DatacenterConfig().Return(c.datacenterConfig),
		c.provider.EXPECT().MachineConfigs().Return(c.machineConfigs),
		c.writer.EXPECT().Write("cluster-name-eks-a-cluster.yaml", gomock.Any(), gomock.Any()),
	)
}

func (c *createTestSetup) expectDeleteBootstrap() {
	c.bootstrapper.EXPECT().DeleteBootstrapCluster(c.ctx, c.bootstrapCluster, gomock.Any())
}

func (c *createTestSetup) expectInstallMHC() {
	gomock.InOrder(
		c.clusterManager.EXPECT().InstallMachineHealthChecks(
			c.ctx, c.bootstrapCluster, c.provider,
		),
	)
}

func (c *createTestSetup) run() error {
	return c.workflow.Run(c.ctx, c.clusterSpec, c.forceCleanup)
}

func TestCreateRunSuccess(t *testing.T) {
	test := newCreateTest(t)
	test.expectSetup()
	test.expectCreateBootstrap()
	test.expectCreateWorkload()
	test.expectMoveManagement()
	test.expectInstallEksaComponents()
	test.expectInstallAddonManager()
	test.expectWriteClusterConfig()
	test.expectDeleteBootstrap()
	test.expectInstallMHC()

	err := test.run()
	if err != nil {
		t.Fatalf("Create.Run() err = %v, want err = nil", err)
	}
}

func TestCreateRunSuccessForceCleanup(t *testing.T) {
	test := newCreateTest(t)
	test.forceCleanup = true
	test.bootstrapper.EXPECT().DeleteBootstrapCluster(test.ctx, &types.Cluster{Name: "cluster-name"}, gomock.Any())
	test.expectSetup()
	test.expectCreateBootstrap()
	test.expectCreateWorkload()
	test.expectMoveManagement()
	test.expectInstallEksaComponents()
	test.expectInstallAddonManager()
	test.expectWriteClusterConfig()
	test.expectDeleteBootstrap()
	test.expectInstallMHC()

	err := test.run()
	if err != nil {
		t.Fatalf("Create.Run() err = %v, want err = nil", err)
	}
}
