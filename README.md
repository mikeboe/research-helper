# ResearchHelper CLI

ResearchHelper-CLI is an autonomous terminal-based research agent written in Go. It researches a thesis topic by iterating through a "Plan-Execute-Reflect" loop, utilizing internal tools (searching/scraping) and an integrated RAG (Retrieval-Augmented Generation) system for indexing knowledge.

## Features

- **Autonomous Research Loop:**
    - **Plan:** Generates specific search queries using an LLM (Google Gemini).
    - **Execute:** Searches external sources (Arxiv, Brave, etc.) via internal Go tools.
    - **Filter & Index:** Filters results and indexes them into a vector database (PostgreSQL/pgvector).
    - **Reflect:** Decides whether to continue researching or finalize the report.
- **Interactive & Non-Interactive Modes:**
    - Run interactively with prompts.
    - Run via command-line flags for automation.
- **Integrated RAG:**
    - Directly indexes findings into the database for retrieval during the chat/report phase.
- **Structured Outputs:** Uses JSON Schema with the LLM to ensure reliable query generation.

## Prerequisites

- **Go 1.22+**
- **Make** (optional, for build commands)
- **PostgreSQL** (with pgvector extension)

## Configuration

The application is configured via environment variables. Create a `.env` file in the root directory:

```env
# Required for the Main Agent Logic
GOOGLE_API_KEY=your_gemini_api_key

# Required for Research Tools
BRAVE_API_TOKEN=your_brave_search_token
MISTRAL_API_KEY=your_mistral_api_key
ANTHROPIC_API_KEY=your_anthropic_key # If used by other tools

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=research_agent
```

## Installation & Build

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/mikeboe/research-helper.git
    cd research-helper
    ```

2.  **Build the CLI:**
    ```bash
    make build
    ```
    This creates the binary at `bin/research-helper`.

3.  **Build the Server:**
    ```bash
    make build-server
    ```
    This creates the binary at `bin/research-server`.

## Usage

### 1. Interactive Mode
Simply run the binary without arguments. You will be prompted for the topic and the collection name.

```bash
./bin/research-helper
```

### 2. Non-Interactive Mode (Flags)
Use flags to automate the process.

```bash
./bin/research-helper --topic "Impact of LoRA on LLM Fine-tuning" --collection "thesis_v1"
```

*   `--topic`, `-t`: The research topic (required in non-interactive mode).
*   `--collection`, `-c`: The target RAG collection name (defaults to "thesis_db").

## Development

*   **Run Tests:** `make test`
*   **Clean:** `make clean`
*   **Run Server:** `make server`
*   **Run Web UI:** `make web`
