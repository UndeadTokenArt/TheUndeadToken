# The Undead Token Homepage

## Description
This project is a web application designed to serve my homepage. It includes various features such as routing, templates, and static assets like CSS and JavaScript files. The application is built using Go and follows a modular structure for better maintainability.

## Motivation
The motivation behind this project is to create a customizable and efficient homepage that I can be used as a starting point for web development projects. It demonstrates the use of Go for backend development, along with HTML templates and static assets for the frontend.

## Quick Start

### Prerequisites
- Go (version 1.20 or higher)
- Docker (optional, for containerized deployment)

### Steps
1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd Homepage
   ```
2. Install dependencies:
   ```bash
   go mod tidy
   ```
3. Run the application:
   ```bash
   go run main.go
   ```
4. Open your browser and navigate to `http://localhost:8080`.

### Docker (Optional)
To run the application in a Docker container:
1. Build the Docker image:
   ```bash
   docker build -t homepage .
   ```
2. Run the Docker container:
   ```bash
   docker run -d -p 127.0.0.1:8080:8080 homepage
   ```

## Usage
- The homepage serves as a starting point for web applications.
- Static assets like CSS and JavaScript files are located in the `static/` directory.
- Templates for dynamic content are located in the `templates/` directory.
- Routes and application logic are defined in the Go files under the root and `internal/` directories.

## Contributing
Contributions are welcome! To contribute:
1. Fork the repository.
2. Create a new branch for your feature or bugfix:
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. Commit your changes:
   ```bash
   git commit -m "Add your commit message"
   ```
4. Push to your branch:
   ```bash
   git push origin feature/your-feature-name
   ```
5. Open a pull request.

## License
This project is licensed under the MIT License. See the LICENSE file for details.