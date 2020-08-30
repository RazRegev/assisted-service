import os
import utils
import deployment_options


def main():
    deploy_options = deployment_options.load_deployment_options()

    utils.verify_build_directory(deploy_options.namespace)
    utils.set_profile(deploy_options.target, deploy_options.profile)

    src_file = os.path.join(os.getcwd(), 'deploy/roles/default_role.yaml')
    dst_file = os.path.join(os.getcwd(), 'build', deploy_options.namespace, 'default_role.yaml')

    with open(src_file, "r") as src:
        with open(dst_file, "w+") as dst:
            data = src.read()
            data = data.replace('REPLACE_NAMESPACE', deploy_options.namespace)
            print("Deploying {}".format(dst_file))
            dst.write(data)

    utils.apply(dst_file)


if __name__ == "__main__":
    main()
