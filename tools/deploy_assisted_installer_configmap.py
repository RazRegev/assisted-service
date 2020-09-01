import os
import utils
import argparse
import yaml
import deployment_options


def handle_arguments():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-dns-domains")
    parser.add_argument("--enable-auth", default="False")
    parser.add_argument("--subsystem-test", action='store_true')
    parser.add_argument("--jwks-url", default="https://api.openshift.com/.well-known/jwks.json")
    parser.add_argument("--ocm-url", default="https://api-integration.6943.hive-integration.openshiftapps.com")
    parser.add_argument("--installation-timeout", type=int)

    return deployment_options.load_deployment_options(parser)


deploy_options = handle_arguments()

SRC_FILE = os.path.join(os.getcwd(), 'deploy/assisted-service-configmap.yaml')
DST_FILE = os.path.join(os.getcwd(), 'build', deploy_options.namespace, 'assisted-service-configmap.yaml')
SERVICE = "assisted-service"


def get_deployment_tag(args):
    if args.deploy_manifest_tag:
        return args.deploy_manifest_tag
    if args.deploy_tag:
        return args.deploy_tag


def main():
    log = utils.get_logger('deploy-service-configmap')
    utils.verify_build_directory(deploy_options.namespace)

    with open(SRC_FILE, 'r') as fp:
        doc = yaml.safe_load(fp)

    data = doc['data']
    data['SERVICE_BASE_URL'] = utils.get_service_url(
        service=SERVICE,
        target=deploy_options.target,
        domain=deploy_options.domain,
        namespace=deploy_options.namespace,
        profile=deploy_options.profile,
        disable_tls=deploy_options.disable_tls
    )
    data['BASE_DNS_DOMAINS'] = deploy_options.base_dns_domains
    data['NAMESPACE'] = deploy_options.namespace
    data['ENABLE_AUTH'] = deploy_options.enable_auth
    data['JWKS_URL'] = deploy_options.jwks_url
    data['OCM_BASE_URL'] = deploy_options.ocm_url
    if deploy_options.installation_timeout:
        data['INSTALLATION_TIMEOUT'] = deploy_options.installation_timeout

    subsystem_versions = {"IMAGE_BUILDER": "ISO_CREATION",
                          "IGNITION_GENERATE_IMAGE": "DUMMY_IGNITION"}

    versions = {"IMAGE_BUILDER": "assisted-iso-create",
                "IGNITION_GENERATE_IMAGE": "assisted-ignition-generator",
                "INSTALLER_IMAGE": "assisted-installer",
                "CONTROLLER_IMAGE": "assisted-installer-controller",
                "AGENT_DOCKER_IMAGE": "assisted-installer-agent",
                "CONNECTIVITY_CHECK_IMAGE": "assisted-installer-agent",
                "INVENTORY_IMAGE": "assisted-installer-agent"}
    for env_var_name, image_short_name in versions.items():
        if deploy_options.subsystem_test and env_var_name in subsystem_versions.keys():
            image_fqdn = deployment_options.get_image_override(deploy_options, image_short_name, subsystem_versions[env_var_name])
        else:
            image_fqdn = deployment_options.get_image_override(deploy_options, image_short_name, env_var_name)
        versions[env_var_name] = image_fqdn

    # Edge case for controller image override
    if os.environ.get("INSTALLER_IMAGE") and not os.environ.get("CONTROLLER_IMAGE"):
        versions["CONTROLLER_IMAGE"] = deployment_options.IMAGE_FQDN_TEMPLATE.format("assisted-installer-controller",
            deployment_options.get_tag(versions["INSTALLER_IMAGE"]))

    versions["SELF_VERSION"] = deployment_options.get_image_override(deploy_options, "assisted-service", "SERVICE")
    deploy_tag = get_deployment_tag(deploy_options)
    if deploy_tag:
        versions["RELEASE_TAG"] = deploy_tag

    data.update(versions)

    utils.set_namespace_in_yaml_docs(docs=[doc], ns=deploy_options.namespace)

    with open(DST_FILE, 'w') as fp:
        yaml.safe_dump(doc, fp)

    log.info('Deploying: %s', DST_FILE)

    utils.apply(
        target=deploy_options.target,
        namespace=deploy_options.namespace,
        profile=deploy_options.profile,
        file=DST_FILE
    )


if __name__ == "__main__":
    main()
