#!/bin/bash

EKS_CLUSTER_NAME=$1
IMAGE_VERSION=$2

if [ -z "$AWS_REGION" ]; then
    AWS_REGION="us-west-2"
fi

AWS_ACCOUNT_ID=$(aws --region $AWS_REGION sts get-caller-identity --query Account --output text)

ECR_REGISTRY_NAME="driver"
ECR_CRD_REGISTRY_NAME="driver-crds"
ECR_REGISTRY_URL="$AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com"
ECR_REGISTRY_URL_WITH_NAME="$ECR_REGISTRY_URL/$ECR_REGISTRY_NAME"
ECR_CRD_REGISTRY_URL_WITH_NAME="$ECR_REGISTRY_URL/$ECR_CRD_REGISTRY_NAME"

IMAGE_TAG=$ECR_REGISTRY_URL_WITH_NAME:$IMAGE_VERSION
CRD_IMAGE_TAG=$ECR_CRD_REGISTRY_URL_WITH_NAME:$IMAGE_VERSION
NAMESPACE="kube-system"
AWS_SERVICE_ACCOUNT_NAME="basic-test-mount-sa"

aws --region $AWS_REGION ecr get-login-password | docker login --username AWS --password-stdin $ECR_REGISTRY_URL
REGISTRY=$ECR_REGISTRY_URL make container
docker push $IMAGE_TAG
docker push $CRD_IMAGE_TAG

eksctl create cluster --name $EKS_CLUSTER_NAME --node-type m5.large --region $AWS_REGION 
eksctl utils associate-iam-oidc-provider --name $EKS_CLUSTER_NAME --approve --region $AWS_REGION
eksctl create iamserviceaccount \
     --name $AWS_SERVICE_ACCOUNT_NAME \
     --namespace $NAMESPACE \
     --cluster $EKS_CLUSTER_NAME \
     --attach-policy-arn arn:aws:iam::aws:policy/AmazonSSMReadOnlyAccess \
     --attach-policy-arn arn:aws:iam::aws:policy/SecretsManagerReadWrite \
     --override-existing-serviceaccounts \
     --approve \
     --region $AWS_REGION

if [[ -z "${RELEASE}" ]]; then
  REGISTRY=$ECR_REGISTRY_URL make e2e-helm-deploy
else
  REGISTRY=$ECR_REGISTRY_URL make e2e-helm-deploy-release
fi
