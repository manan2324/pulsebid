.PHONY: web-install web-dev web-build web-start web-lint web-typecheck web-test web-e2e web-report web-gen-api
web-install:
	npm --prefix frontend $(if $(CI),ci,install)
web-dev:
	npm --prefix frontend run dev
web-build:
	npm --prefix frontend run build
web-start:
	npm --prefix frontend run start
web-lint:
	npm --prefix frontend run lint
web-typecheck:
	npm --prefix frontend run typecheck
web-test:
	npm --prefix frontend run test
web-e2e:
	npm --prefix frontend run e2e
web-report:
	npm --prefix frontend run report
web-gen-api:
	npm --prefix frontend run gen:api
.PHONY: web-check-api web-check-bundles web-check-secrets web-coverage web-analyze web-lighthouse web-archive
web-check-api:
	npm --prefix frontend run check:api
web-check-bundles:
	npm --prefix frontend run check:bundles
web-check-secrets:
	npm --prefix frontend run check:secrets
web-coverage:
	npm --prefix frontend run test:coverage
web-analyze:
	npm --prefix frontend run analyze
web-lighthouse:
	npm --prefix frontend run lighthouse
web-archive:
	npm --prefix frontend run report:archive
