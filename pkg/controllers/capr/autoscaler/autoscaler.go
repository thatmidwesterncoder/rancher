package autoscaler

import (
	"context"
	"log"
	"runtime"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/wrangler"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Register(ctx context.Context, clients *wrangler.Context) {
	userManager, err := common.NewUserManagerNoBindings(clients)
	if err != nil {
		runtime.Breakpoint()
		log.Fatal(err)
	}
	// Create user for the autoscaler
	user, err := clients.Mgmt.User().Create(&v3.User{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "autoscaler-",
		},
		Username: "autoscaler-test-scale",
	})
	if err != nil {
		runtime.Breakpoint()
		log.Fatal(err)
	}

	// Create GlobalRole with NamespacedRules for fleet-default namespace
	_, err = clients.Mgmt.GlobalRole().Create(&v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "autoscaler-global-role",
		},
		DisplayName: "Autoscaler Global Role",
		NamespacedRules: map[string][]rbacv1.PolicyRule{
			"fleet-default": {
				{
					APIGroups:     []string{"cluster.x-k8s.io"},
					ResourceNames: []string{"test-scale-pool1"},
					Resources:     []string{"machinedeployments"},
					Verbs:         []string{"update"},
				},
				{
					APIGroups: []string{"cluster.x-k8s.io"},
					Resources: []string{
						"machinedeployments",
						"machinepools",
						"machines",
						"machinesets",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups:     []string{"cluster.x-k8s.io"},
					Resources:     []string{"machinedeployments/scale"},
					ResourceNames: []string{"test-scale-pool1"},
					Verbs:         []string{"get", "patch", "update"},
				},
			},
		},
	})
	// if err != nil k8serr.IsAl{
	// 	runtime.Breakpoint()
	// 	log.Fatal(err)
	// }

	// Create GlobalRoleBinding
	_, err = clients.Mgmt.GlobalRoleBinding().Create(&v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "autoscaler-global-rolebinding-",
		},
		GlobalRoleName: "autoscaler-global-role",
		UserName:       user.ObjectMeta.Name,
	})
	if err != nil {
		runtime.Breakpoint()
		log.Fatal(err)
	}
}
