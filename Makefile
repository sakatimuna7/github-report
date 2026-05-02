BINARY_NAME=ghreport
INSTALL_PATH=/usr/local/bin

.PHONY: all build install clean run

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) main.go

install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	@sudo mv $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Successfully installed $(BINARY_NAME)!"

clean:
	@echo "Cleaning up..."
	@rm -f $(BINARY_NAME)

run: build
	@./$(BINARY_NAME)
