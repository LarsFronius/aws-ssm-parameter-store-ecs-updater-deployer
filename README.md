# aws-ssm-parameter-store-ecs-updater-deployer

# Parameter Store ECS Updater / Deployer

AWS Systems Manager (SSM) Parameter Store values can be referenced and used in ECS environment variables via the `ValueFrom` attribute. 

Parameter Store values are read on task creation and thus subsequent updates to the Parameter Store parameter require a restart or redeploy of ECS tasks.

This serverless application facilitates propagating Parameter Store updates via Cloudwatch Events to a Lambda function, that will restart specified ECS services.

Services to be restarted on a parameter update can be specified on the parameter itself via the `restarts` tag as a comma-seperated list of `<cluster>:<service>`.

Made with ❤️ by Lars Fronius. Available on the [AWS Serverless Application Repository](https://aws.amazon.com/serverless)

