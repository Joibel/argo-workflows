GIT_TAG               := $(shell git describe --exact-match --tags --abbrev=0 2> /dev/null || echo untagged)
ifeq ($(GIT_TAG),untagged)
# "SNAPSHOT" is "latest" for Java
VERSION               := 0.0.0-SNAPSHOT
else
# remove the "v" prefix, not allowed
VERSION               := $(GIT_TAG:v=)
endif

# work dir
WD                    := $(shell echo "`pwd`/client")

# Maven, the JDK and the generator all come from the development shell (see
# flake.nix), which pins the version the published client is generated with.
TOOL_OPENAPI_GENERATOR := openapi-generator-cli
MVN                    := mvn

# The published client is for Java 8 consumers, but the shell's JDK is current.
# `release` compiles against that JDK's Java 8 API signatures, which is what the
# `source`/`target` 1.8 in the generated POM meant back when publishing ran on a
# JDK 8 container; `source`/`target` alone would let a modern API be linked into
# Java 8 bytecode. Only `publish` sets it: `install` and `test` below are also
# run from CI on whatever JDK that job provides, and `release` needs a JDK 9+.
JAVA_RELEASE          := 8

publish: generate
	# https://help.github.com/en/packages/using-github-packages-with-your-projects-ecosystem/configuring-apache-maven-for-use-with-github-packages
	$(MVN) -f client -s settings.xml -Dmaven.compiler.release=$(JAVA_RELEASE) deploy -DskipTests -DaltDeploymentRepository=github::default::https://maven.pkg.github.com/argoproj/argo-workflows

generate:
	rm -Rf $(WD)
	mkdir -p $(WD)
	cat ../../api/openapi-spec/swagger.json | \
		sed 's/io.k8s.api.core.v1.//' | \
		sed 's/io.k8s.apimachinery.pkg.apis.meta.v1.//' \
		> $(WD)/swagger.json
	$(TOOL_OPENAPI_GENERATOR) \
		generate \
		-i $(WD)/swagger.json \
		-g java \
		-o $(WD) \
		-p hideGenerationTimestamp=true \
		-p serializationLibrary=jsonb \
		-p dateLibrary=java8 \
		--api-package io.argoproj.workflow.apis \
		--invoker-package io.argoproj.workflow \
		--model-package io.argoproj.workflow.models \
		--skip-validate-spec \
		--group-id io.argoproj.workflow \
		--artifact-id argo-client-java \
		--artifact-version $(VERSION) \
		--import-mappings Time=java.time.Instant \
		--import-mappings Affinity=io.kubernetes.client.openapi.models.V1Affinity \
		--import-mappings ConfigMapKeySelector=io.kubernetes.client.openapi.models.V1ConfigMapKeySelector \
		--import-mappings Container=io.kubernetes.client.openapi.models.V1Container \
		--import-mappings ContainerPort=io.kubernetes.client.openapi.models.V1ContainerPort \
		--import-mappings EnvFromSource=io.kubernetes.client.openapi.models.V1EnvFromSource \
		--import-mappings EnvVar=io.kubernetes.client.openapi.models.V1EnvVar \
		--import-mappings HostAlias=io.kubernetes.client.openapi.models.V1HostAlias \
		--import-mappings Lifecycle=io.kubernetes.client.openapi.models.V1Lifecycle \
		--import-mappings ListMeta=io.kubernetes.client.openapi.models.V1ListMeta \
		--import-mappings LocalObjectReference=io.kubernetes.client.openapi.models.V1LocalObjectReference \
		--import-mappings ObjectMeta=io.kubernetes.client.openapi.models.V1ObjectMeta \
		--import-mappings ObjectReference=io.kubernetes.client.openapi.models.V1ObjectReference \
		--import-mappings PersistentVolumeClaim=io.kubernetes.client.openapi.models.V1PersistentVolumeClaim \
		--import-mappings PodDisruptionBudgetSpec=io.kubernetes.client.openapi.models.V1beta1PodDisruptionBudgetSpec \
		--import-mappings PodDNSConfig=io.kubernetes.client.openapi.models.V1PodDNSConfig \
		--import-mappings PodSecurityContext=io.kubernetes.client.openapi.models.V1PodSecurityContext \
		--import-mappings Probe=io.kubernetes.client.openapi.models.V1Probe \
		--import-mappings ResourceRequirements=io.kubernetes.client.openapi.models.V1ResourceRequirements \
		--import-mappings SecretKeySelector=io.kubernetes.client.openapi.models.V1SecretKeySelector \
		--import-mappings SecurityContext=io.kubernetes.client.openapi.models.V1SecurityContext \
		--import-mappings Toleration=io.kubernetes.client.openapi.models.V1Toleration \
		--import-mappings Volume=io.kubernetes.client.openapi.models.V1Volume \
		--import-mappings VolumeDevice=io.kubernetes.client.openapi.models.V1VolumeDevice \
		--import-mappings VolumeMount=io.kubernetes.client.openapi.models.V1VolumeMount \
		--generate-alias-as-model
	# adding kubernetes-client
	cd client && sed 's/<dependencies>/<dependencies><dependency><groupId>io.kubernetes<\/groupId><artifactId>client-java<\/artifactId><version>14.0.1<\/version><\/dependency>/g' pom.xml > tmp && mv tmp pom.xml

client/pom.xml: generate

install: client/pom.xml
	$(MVN) -f client install

test:
	../../hack/access-token.sh init
	kubectl delete wf --all
	env ARGO_TOKEN="`../../hack/access-token.sh get`" $(MVN) -f tests test
