# Tour Assist

![](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white) ![](https://img.shields.io/badge/OpenAI-412991?logo=openai&logoColor=white) ![](https://img.shields.io/badge/MongoDB-47A248?logo=mongodb&logoColor=white)

> AI-powered travel assistant API with conversational interface

## About

Travel Assist is a personal assistant service that provides an API for conversations with an AI assistant specialized for travel. Originally developed as part of a technical challenge for a Software Engineer position, this project showcases modern AI integration patterns and RESTful API design.

## Description

Travel Assist offers a ChatGPT-like conversational interface through a RESTful API. The application manages multi-turn conversations with an AI assistant, storing conversation history and providing intelligent responses powered by OpenAI's language models.

### Key Features

- **Conversational AI**: Start new conversations, send messages, and retrieve conversation history
- **Real-time Weather Information**: Get current weather conditions and forecasts for any location
- **Date and Time Queries**: Ask about current date, time, and time zones
- **Barcelona Holiday Information**: Access information about holidays in Barcelona
- **General AI Assistance**: Leverage OpenAI's powerful language models for general queries
- **Persistent Storage**: All conversations are stored in MongoDB for retrieval
- **HTTP-based API**: Simple JSON-based API built with Twirp and Protocol Buffers

## Built With

- Go
- OpenAI API
- MongoDB
- Twirp/Protobuf

## Getting Started

To get a local copy up and running follow these simple steps.

### Prerequisites

- [Go](https://go.dev/doc/install) (v1.24 or later recommended)
- [Docker](https://docs.docker.com/get-docker/) (to run MongoDB container)
- Git and Make
- OpenAI API key ([get one here](https://platform.openai.com/api-keys))

### Setup

- Go to the main page for this project: https://github.com/lu-jim/travel-assist
- Click the green Code button
- Copy the repository URL
- Open your terminal and change to your desired directory

### Install

```sh
git clone https://github.com/lu-jim/travel-assist.git
cd travel-assist
go mod download
```

## Run

```sh
# Start MongoDB container
make up

# Set your OpenAI API key
export OPENAI_API_KEY=your_openai_api_key

# Run the server
make run
```

The server will start at [localhost:8080](http://localhost:8080).

To stop the application:
- Press `Ctrl+C` to stop the server
- Run `make down` to stop the MongoDB container

## Usage

The application provides a simple HTTP-based API. You can interact with it using:

- **HTTP Client**: Use curl, Postman, or any HTTP client with the [Postman collection](https://documenter.getpostman.com/view/40257649/2sB3BKFo8S)
- **CLI Tool**: Use the included [CLI tool](cmd/cli/README.md) for easy interaction

### API Endpoints

- `POST /twirp/rpc.ChatService/StartConversation` - Start a new conversation
- `POST /twirp/rpc.ChatService/SendMessage` - Send a message to an existing conversation
- `POST /twirp/rpc.ChatService/GetConversation` - Retrieve a conversation by ID
- `POST /twirp/rpc.ChatService/ListConversations` - List all conversations

## Testing

The codebase includes comprehensive tests for the server and assistant functionality.

```sh
# Make sure MongoDB is running
make up

# Run all tests
go test ./...
```

## Authors

👤 **Luis Fernando Jimenez**

- GitHub: [@lu-jim](https://github.com/lu-jim)
- Twitter: [@lujimhe](https://twitter.com/lujimhe)
- LinkedIn: [@lujim](https://www.linkedin.com/in/lujim/)

## Contributing

Contributions, issues, and feature requests are welcome!

[Open an issue here](https://github.com/lu-jim/travel-assist/issues/new).

## Show your support

Give a ⭐️ if you like this project!

## Acknowledgments

Special thanks to [Acai Travel](https://acaitravel.com) for the original project structure and technical challenge guidelines that formed the foundation of this application.

---
