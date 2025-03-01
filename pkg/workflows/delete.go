package workflows

import (
	"context"

	"github.com/aws/eks-anywhere/pkg/cluster"
	"github.com/aws/eks-anywhere/pkg/logger"
	"github.com/aws/eks-anywhere/pkg/providers"
	"github.com/aws/eks-anywhere/pkg/task"
	"github.com/aws/eks-anywhere/pkg/types"
	"github.com/aws/eks-anywhere/pkg/workflows/interfaces"
)

type Delete struct {
	bootstrapper   interfaces.Bootstrapper
	provider       providers.Provider
	clusterManager interfaces.ClusterManager
	addonManager   interfaces.AddonManager
}

func NewDelete(bootstrapper interfaces.Bootstrapper, provider providers.Provider,
	clusterManager interfaces.ClusterManager, addonManager interfaces.AddonManager) *Delete {
	return &Delete{
		bootstrapper:   bootstrapper,
		provider:       provider,
		clusterManager: clusterManager,
		addonManager:   addonManager,
	}
}

func (c *Delete) Run(ctx context.Context, workloadCluster *types.Cluster, clusterSpec *cluster.Spec, forceCleanup bool) error {
	if forceCleanup {
		if err := c.bootstrapper.DeleteBootstrapCluster(ctx, &types.Cluster{
			Name: workloadCluster.Name,
		}, false); err != nil {
			return err
		}
	}

	commandContext := &task.CommandContext{
		Bootstrapper:    c.bootstrapper,
		Provider:        c.provider,
		ClusterManager:  c.clusterManager,
		AddonManager:    c.addonManager,
		WorkloadCluster: workloadCluster,
		ClusterSpec:     clusterSpec,
		Rollback:        false,
	}
	err := task.NewTaskRunner(&setupAndValidate{}).RunTask(ctx, commandContext)
	if err != nil {
		_ = commandContext.ClusterManager.SaveLogs(ctx, commandContext.BootstrapCluster)
	}
	return err
}

type setupAndValidate struct{}

type createManagementCluster struct{}

type installCapi struct{}

type moveClusterManagement struct{}

type deleteWorkloadCluster struct{}

type cleanupGitRepo struct{}

type deleteManagementCluster struct{}

type cleanupProviderInfrastructure struct{}

func (s *setupAndValidate) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Performing provider setup and validations")
	err := commandContext.Provider.SetupAndValidateDeleteCluster(ctx)
	if err != nil {
		commandContext.SetError(err)
		return nil
	}
	return &createManagementCluster{}
}

func (s *setupAndValidate) Name() string {
	return "setup-and-validate"
}

func (s *createManagementCluster) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Creating management cluster")
	bootstrapOptions, err := commandContext.Provider.BootstrapClusterOpts()
	if err != nil {
		logger.Error(err, "Error getting management options from provider")
		commandContext.SetError(err)
		return nil
	}

	bootstrapCluster, err := commandContext.Bootstrapper.CreateBootstrapCluster(ctx, commandContext.ClusterSpec, bootstrapOptions...)
	if err != nil {
		commandContext.SetError(err)
		return &deleteManagementCluster{}
	}
	commandContext.BootstrapCluster = bootstrapCluster

	return &installCapi{}
}

func (s *createManagementCluster) Name() string {
	return "management-cluster-init"
}

func (s *installCapi) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Installing cluster-api providers on management cluster")
	err := commandContext.ClusterManager.InstallCapi(ctx, commandContext.ClusterSpec, commandContext.BootstrapCluster, commandContext.Provider)
	if err != nil {
		commandContext.SetError(err)
		return &deleteManagementCluster{}
	}
	return &moveClusterManagement{}
}

func (s *installCapi) Name() string {
	return "install-capi"
}

func (s *moveClusterManagement) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Moving cluster management from workload cluster")
	err := commandContext.ClusterManager.MoveCapi(ctx, commandContext.WorkloadCluster, commandContext.BootstrapCluster, types.WithNodeRef())
	if err != nil {
		commandContext.SetError(err)
		return nil
	}
	return &deleteWorkloadCluster{}
}

func (s *moveClusterManagement) Name() string {
	return "cluster-management-move"
}

func (s *deleteWorkloadCluster) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Deleting workload cluster")
	err := commandContext.ClusterManager.DeleteCluster(ctx, commandContext.BootstrapCluster, commandContext.WorkloadCluster)
	if err != nil {
		commandContext.SetError(err)
		return nil
	}

	return &cleanupGitRepo{}
}

func (s *deleteWorkloadCluster) Name() string {
	return "delete-workload-cluster"
}

func (s *cleanupGitRepo) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Clean up Git Repo")
	err := commandContext.AddonManager.CleanupGitRepo(ctx, commandContext.ClusterSpec)
	if err != nil {
		commandContext.SetError(err)
		return nil
	}

	return &deleteManagementCluster{}
}

func (s *cleanupGitRepo) Name() string {
	return "clean-up-git-repo"
}

func (s *deleteManagementCluster) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	if commandContext.BootstrapCluster != nil {
		if err := commandContext.Bootstrapper.DeleteBootstrapCluster(ctx, commandContext.BootstrapCluster, false); err != nil {
			commandContext.SetError(err)
		}
		return &cleanupProviderInfrastructure{}
	}
	logger.Info("Bootstrap cluster information missing - skipping delete kind cluster")
	return &cleanupProviderInfrastructure{}
}

func (s *deleteManagementCluster) Name() string {
	return "kind-cluster-delete"
}

func (s *cleanupProviderInfrastructure) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	err := commandContext.Provider.CleanupProviderInfrastructure(ctx)
	if err != nil {
		commandContext.SetError(err)
		return nil
	}
	if commandContext.OriginalError == nil {
		logger.MarkSuccess("Cluster deleted!")
	}
	return nil
}

func (s *cleanupProviderInfrastructure) Name() string {
	return "cleanup-provider-infrastructure"
}
