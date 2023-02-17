.PHONY: docs

# generate SVG images for docs
# (requires graphviz)
docs:
	go run cmd/main.go compile -f docs/examples/overview/workflow.yml -s docs/examples/overview/schema.json | dot -Tsvg > docs/img/overview-compiled.svg
	go run cmd/main.go run -f docs/examples/overview/workflow.yml -s docs/examples/overview/schema.json -i docs/examples/overview/input-oncall.json | dot -Tsvg > docs/img/overview-oncall.svg
	go run cmd/main.go run -f docs/examples/overview/workflow.yml -s docs/examples/overview/schema.json -i docs/examples/overview/input-not-oncall.json | dot -Tsvg > docs/img/overview-not-oncall.svg
	go run cmd/main.go run -f docs/examples/overview-execution/workflow.yml -s docs/examples/overview-execution/schema.json -i docs/examples/overview-execution/input.json | dot -Tsvg > docs/img/overview-execution.svg
	go run cmd/main.go run -f docs/examples/overview-approval/workflow.yml -s docs/examples/overview-approval/schema.json -i docs/examples/overview-approval/input.json | dot -Tsvg > docs/img/overview-approval.svg
	go run cmd/main.go run -f docs/examples/overview-approval/workflow.yml -s docs/examples/overview-approval/schema.json -i docs/examples/overview-approval/input-approved.json | dot -Tsvg > docs/img/overview-approval-complete.svg
	go run cmd/main.go compile -f docs/examples/overview-multi/workflow.yml -s docs/examples/overview-multi/schema.json | dot -Tsvg > docs/img/overview-multi.svg

cli:
	go build -o bin/glide cmd/main.go
	mv ./bin/glide /usr/local/bin/