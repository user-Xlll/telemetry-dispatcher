include .env

build:
	docker build \
		--build-arg NEXUS_USERNAME=${NEXUS_USERNAME} \
		--build-arg NEXUS_PASSWORD=${NEXUS_PASSWORD} \
		--tag docker-dev.registry-ci.delta.sbrf.ru/ci00766784/ci15891918/telemetry-dispatcher \
		.
