// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure

import (
	"context"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/pkg/errors"
)

func (a *actuator) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	return a.reconcile(ctx, infra, cluster, terraformer.StateConfigMapInitializerFunc(terraformer.CreateState))
}

func (a *actuator) reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster, stateInitializer terraformer.StateConfigMapInitializer) error {
	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	creds, err := infrastructure.GetCredentialsFromInfrastructure(ctx, a.Client(), infra)
	if err != nil {
		return err
	}

	terraformFiles, err := infrastructure.RenderTerraformerChart(a.ChartRenderer(), infra, creds, config, cluster)
	if err != nil {
		return err
	}

	additionalEnvs := make(map[string]string)
	if cluster.Shoot.Spec.Networking.ProxyConfig != nil {
		if cluster.Shoot.Spec.Networking.ProxyConfig.NoProxy != nil {
			additionalEnvs["no_proxy"] = *cluster.Shoot.Spec.Networking.ProxyConfig.NoProxy
		}
		if cluster.Shoot.Spec.Networking.ProxyConfig.HttpProxy != nil {
			additionalEnvs["http_proxy"] = *cluster.Shoot.Spec.Networking.ProxyConfig.HttpProxy
		}
	}

	tf, err := internal.NewTerraformerWithAuth(a.RESTConfig(), infrastructure.TerraformerPurpose, infra.Namespace, infra.Name, creds, additionalEnvs)
	if err != nil {
		return err
	}

	if err := tf.
		InitializeWith(terraformer.DefaultInitializer(a.Client(), terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, stateInitializer)).
		Apply(); err != nil {

		return errors.Wrap(err, "failed to apply the terraform config")
	}

	return a.updateProviderStatus(ctx, tf, infra, config)
}
