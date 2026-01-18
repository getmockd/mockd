package cli

import (
	"flag"
	"fmt"
	"os"
)

// RunCompletion handles the completion command.
func RunCompletion(args []string) error {
	fs := flag.NewFlagSet("completion", flag.ContinueOnError)

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd completion <shell>

Generate shell completion scripts.

Supported shells: bash, zsh, fish

Arguments:
  shell    Target shell (bash, zsh, fish)

Examples:
  # Bash (add to ~/.bashrc or /etc/bash_completion.d/)
  mockd completion bash > /etc/bash_completion.d/mockd
  # Or for user install:
  mockd completion bash >> ~/.bashrc

  # Zsh (add to fpath)
  mockd completion zsh > "${fpath[1]}/_mockd"
  # Or for Oh My Zsh:
  mockd completion zsh > ~/.oh-my-zsh/completions/_mockd

  # Fish
  mockd completion fish > ~/.config/fish/completions/mockd.fish
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf(`shell type is required

Usage: mockd completion <shell>

Supported shells: bash, zsh, fish`)
	}

	shell := fs.Arg(0)
	switch shell {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		return fmt.Errorf("unknown shell: %s\n\nSupported shells: bash, zsh, fish", shell)
	}

	return nil
}

const bashCompletion = `# mockd bash completion
_mockd() {
    local cur prev words cword
    _init_completion || return

    local commands="serve start stop status init new add list get delete import export logs config completion version help doctor proxy recordings convert generate enhance stream-recordings graphql chaos grpc mqtt soap templates tunnel websocket"

    if [[ ${cword} -eq 1 ]]; then
        COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
        return
    fi

    case ${words[1]} in
        serve)
            COMPREPLY=($(compgen -W "--port -p --admin-port -a --config -c --https-port --read-timeout --write-timeout --max-log-entries --auto-cert --tls-cert --tls-key --tls-auto --mtls-enabled --mtls-client-auth --mtls-ca --mtls-allowed-cns --audit-enabled --audit-file --audit-level --register --control-plane --token --name --labels --pull --cache --graphql-schema --graphql-path --grpc-port --grpc-proto --grpc-reflection --oauth-enabled --oauth-issuer --oauth-port --mqtt-port --mqtt-auth --chaos-enabled --chaos-latency --chaos-error-rate --validate-spec --validate-fail --detach -d --pid-file --help" -- "${cur}"))
            ;;
        start)
            COMPREPLY=($(compgen -W "--port -p --admin-port -a --config -c --https-port --read-timeout --write-timeout --max-log-entries --auto-cert --help" -- "${cur}"))
            ;;
        stop)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=($(compgen -W "admin engine --pid-file --force -f --timeout --help" -- "${cur}"))
            else
                COMPREPLY=($(compgen -W "--pid-file --force -f --timeout --help" -- "${cur}"))
            fi
            ;;
        status)
            COMPREPLY=($(compgen -W "--pid-file --json --help" -- "${cur}"))
            ;;
        init)
            COMPREPLY=($(compgen -W "--output -o --force --format --help" -- "${cur}"))
            ;;
        new)
            COMPREPLY=($(compgen -W "--template -t --name -n --output -o --resource --help" -- "${cur}"))
            ;;
        tunnel)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=($(compgen -W "status stop --port -p --admin-port --config -c --relay --token --subdomain -s --domain --auth-token --auth-basic --allow-ips --help" -- "${cur}"))
            else
                COMPREPLY=($(compgen -W "--port -p --admin-port --config -c --relay --token --subdomain -s --domain --auth-token --auth-basic --allow-ips --help" -- "${cur}"))
            fi
            ;;
        add)
            COMPREPLY=($(compgen -W "--type -t --method -m --path --status -s --body -b --body-file --header -H --match-header --match-query --name -n --priority --delay --admin-url --json --message --echo --operation --op-type --response --service --rpc-method --topic --payload --qos --soap-action --help" -- "${cur}"))
            ;;
        list)
            COMPREPLY=($(compgen -W "--config -c --type -t --admin-url --json --help" -- "${cur}"))
            ;;
        get|delete)
            COMPREPLY=($(compgen -W "--admin-url --json --help" -- "${cur}"))
            ;;
        import)
            COMPREPLY=($(compgen -W "--replace --admin-url --help" -- "${cur}"))
            ;;
        export)
            COMPREPLY=($(compgen -W "--output -o --name -n --admin-url --help" -- "${cur}"))
            ;;
        logs)
            COMPREPLY=($(compgen -W "--method -m --path -p --matched --unmatched --limit -n --verbose --clear --admin-url --json --help" -- "${cur}"))
            ;;
        config)
            COMPREPLY=($(compgen -W "--json --help" -- "${cur}"))
            ;;
        proxy)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=($(compgen -W "start stop status mode ca --help" -- "${cur}"))
            else
                case ${words[2]} in
                    start)
                        COMPREPLY=($(compgen -W "--port -p --mode -m --session -s --ca-path --include --exclude --include-hosts --exclude-hosts --help" -- "${cur}"))
                        ;;
                    mode)
                        COMPREPLY=($(compgen -W "record passthrough --help" -- "${cur}"))
                        ;;
                    ca)
                        if [[ ${cword} -eq 3 ]]; then
                            COMPREPLY=($(compgen -W "export generate --help" -- "${cur}"))
                        else
                            COMPREPLY=($(compgen -W "--output -o --ca-path --help" -- "${cur}"))
                        fi
                        ;;
                esac
            fi
            ;;
        recordings)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=($(compgen -W "list convert export import clear --help" -- "${cur}"))
            else
                case ${words[2]} in
                    list)
                        COMPREPLY=($(compgen -W "--session --method --path --json --limit --help" -- "${cur}"))
                        ;;
                    convert)
                        COMPREPLY=($(compgen -W "--session --deduplicate --include-headers --output -o --help" -- "${cur}"))
                        ;;
                    export)
                        COMPREPLY=($(compgen -W "--session --output -o --help" -- "${cur}"))
                        ;;
                    import)
                        COMPREPLY=($(compgen -W "--input -i --help" -- "${cur}"))
                        ;;
                    clear)
                        COMPREPLY=($(compgen -W "--force -f --help" -- "${cur}"))
                        ;;
                esac
            fi
            ;;
        convert)
            COMPREPLY=($(compgen -W "--recording --session --path-filter --method --status --smart-match --duplicates --include-headers --output -o --check-sensitive --help" -- "${cur}"))
            ;;
        generate)
            COMPREPLY=($(compgen -W "--input -i --prompt -p --output -o --ai --provider --model --dry-run --admin-url --help" -- "${cur}"))
            ;;
        enhance)
            COMPREPLY=($(compgen -W "--ai --provider --model --admin-url --help" -- "${cur}"))
            ;;
        stream-recordings)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=($(compgen -W "list show delete export convert stats vacuum sessions --help" -- "${cur}"))
            else
                case ${words[2]} in
                    list)
                        COMPREPLY=($(compgen -W "--protocol --path --status --json --limit --offset --sort --order --include-deleted --help" -- "${cur}"))
                        ;;
                    show|get)
                        COMPREPLY=($(compgen -W "--json --help" -- "${cur}"))
                        ;;
                    delete|rm)
                        COMPREPLY=($(compgen -W "--force -f --permanent --help" -- "${cur}"))
                        ;;
                    export)
                        COMPREPLY=($(compgen -W "--output -o --help" -- "${cur}"))
                        ;;
                    convert)
                        COMPREPLY=($(compgen -W "--output -o --simplify-timing --min-delay --max-delay --include-client --deduplicate --help" -- "${cur}"))
                        ;;
                    stats|sessions)
                        COMPREPLY=($(compgen -W "--json --help" -- "${cur}"))
                        ;;
                    vacuum)
                        COMPREPLY=($(compgen -W "--force -f --help" -- "${cur}"))
                        ;;
                esac
            fi
            ;;
        graphql)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=($(compgen -W "validate query --help" -- "${cur}"))
            else
                case ${words[2]} in
                    query)
                        COMPREPLY=($(compgen -W "--variables -v --operation -o --header -H --pretty --help" -- "${cur}"))
                        ;;
                esac
            fi
            ;;
        chaos)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=($(compgen -W "enable disable status --help" -- "${cur}"))
            else
                case ${words[2]} in
                    enable)
                        COMPREPLY=($(compgen -W "--admin-url --latency -l --error-rate -e --error-code --path -p --probability --help" -- "${cur}"))
                        ;;
                    disable|status)
                        COMPREPLY=($(compgen -W "--admin-url --json --help" -- "${cur}"))
                        ;;
                esac
            fi
            ;;
        grpc)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=($(compgen -W "list call --help" -- "${cur}"))
            else
                case ${words[2]} in
                    list)
                        COMPREPLY=($(compgen -W "--import -I --help" -- "${cur}"))
                        ;;
                    call)
                        COMPREPLY=($(compgen -W "--metadata -m --plaintext --pretty --help" -- "${cur}"))
                        ;;
                esac
            fi
            ;;
        mqtt)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=($(compgen -W "publish subscribe status --help" -- "${cur}"))
            else
                case ${words[2]} in
                    publish)
                        COMPREPLY=($(compgen -W "--broker -b --username -u --password -P --qos --retain --help" -- "${cur}"))
                        ;;
                    subscribe)
                        COMPREPLY=($(compgen -W "--broker -b --username -u --password -P --qos --count -n --timeout -t --help" -- "${cur}"))
                        ;;
                    status)
                        COMPREPLY=($(compgen -W "--admin-url --json --help" -- "${cur}"))
                        ;;
                esac
            fi
            ;;
        websocket)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=($(compgen -W "connect send listen status --help" -- "${cur}"))
            else
                case ${words[2]} in
                    connect)
                        COMPREPLY=($(compgen -W "--header -H --subprotocol --timeout -t --json --help" -- "${cur}"))
                        ;;
                    send)
                        COMPREPLY=($(compgen -W "--header -H --subprotocol --timeout -t --json --help" -- "${cur}"))
                        ;;
                    listen)
                        COMPREPLY=($(compgen -W "--header -H --subprotocol --timeout -t --count -n --json --help" -- "${cur}"))
                        ;;
                    status)
                        COMPREPLY=($(compgen -W "--admin-url --json --help" -- "${cur}"))
                        ;;
                esac
            fi
            ;;
        soap)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=($(compgen -W "validate call --help" -- "${cur}"))
            else
                case ${words[2]} in
                    call)
                        COMPREPLY=($(compgen -W "--action -a --body -b --header -H --soap12 --pretty --timeout --help" -- "${cur}"))
                        ;;
                esac
            fi
            ;;
        templates)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=($(compgen -W "list add --help" -- "${cur}"))
            else
                case ${words[2]} in
                    list)
                        COMPREPLY=($(compgen -W "--category -c --base-url --help" -- "${cur}"))
                        ;;
                    add)
                        COMPREPLY=($(compgen -W "--output -o --admin-url --base-url --dry-run --help" -- "${cur}"))
                        ;;
                esac
            fi
            ;;
        completion)
            COMPREPLY=($(compgen -W "bash zsh fish" -- "${cur}"))
            ;;
        version)
            COMPREPLY=($(compgen -W "--json --help" -- "${cur}"))
            ;;
        help)
            COMPREPLY=($(compgen -W "config matching templating formats websocket graphql grpc mqtt soap sse" -- "${cur}"))
            ;;
        doctor)
            COMPREPLY=($(compgen -W "--config -c --port -p --admin-port -a --help" -- "${cur}"))
            ;;
        init)
            COMPREPLY=($(compgen -W "--output -o --force --format --interactive -i --template -t --help" -- "${cur}"))
            ;;
    esac
}

complete -F _mockd mockd
`

const zshCompletion = `#compdef mockd

_mockd() {
    local -a commands
    commands=(
        'serve:Start the mock server (default command)'
        'start:Start the mock server (alias for serve)'
        'stop:Stop a running mockd server'
        'status:Show status of running mockd server'
        'init:Create a starter config file'
        'new:Create mocks from templates'
        'add:Add a new mock endpoint'
        'list:List all configured mocks'
        'get:Get details of a specific mock'
        'delete:Delete a mock by ID'
        'import:Import mocks from a configuration file'
        'export:Export current mocks to stdout or file'
        'logs:View request logs'
        'config:Show effective configuration'
        'proxy:Manage the MITM proxy for recording'
        'recordings:Manage recorded API traffic'
        'convert:Convert recordings to mock definitions'
        'generate:Generate mocks from OpenAPI or AI'
        'enhance:Enhance mocks with AI-generated data'
        'stream-recordings:Manage WebSocket/SSE recordings'
        'graphql:Validate schemas and execute queries'
        'chaos:Manage chaos injection'
        'grpc:Manage and test gRPC endpoints'
        'mqtt:Publish, subscribe, and manage MQTT'
        'websocket:Connect, send, and listen to WebSocket endpoints'
        'soap:Validate WSDL and call SOAP operations'
        'templates:List and add templates from library'
        'tunnel:Expose local mocks via cloud relay'
        'completion:Generate shell completion scripts'
        'version:Show version information'
        'help:Show help for topics'
        'doctor:Diagnose common setup issues'
    )

    if (( CURRENT == 2 )); then
        _describe -t commands 'mockd commands' commands
        return
    fi

    case ${words[2]} in
        serve)
            _arguments \
                '(-p --port)'{-p,--port}'[HTTP server port]:port:' \
                '(-a --admin-port)'{-a,--admin-port}'[Admin API port]:port:' \
                '(-c --config)'{-c,--config}'[Path to mock configuration file]:file:_files' \
                '--https-port[HTTPS server port]:port:' \
                '--read-timeout[Read timeout in seconds]:seconds:' \
                '--write-timeout[Write timeout in seconds]:seconds:' \
                '--max-log-entries[Maximum request log entries]:count:' \
                '--auto-cert[Auto-generate TLS certificate]' \
                '--tls-cert[Path to TLS certificate file]:file:_files' \
                '--tls-key[Path to TLS private key file]:file:_files' \
                '--tls-auto[Auto-generate self-signed certificate]' \
                '--mtls-enabled[Enable mTLS client certificate validation]' \
                '--mtls-client-auth[Client auth mode]:mode:(none request require verify-if-given require-and-verify)' \
                '--mtls-ca[Path to CA certificate]:file:_files' \
                '--mtls-allowed-cns[Comma-separated allowed Common Names]:cns:' \
                '--audit-enabled[Enable audit logging]' \
                '--audit-file[Path to audit log file]:file:_files' \
                '--audit-level[Log level]:level:(debug info warn error)' \
                '--register[Register with control plane as runtime]' \
                '--control-plane[Control plane URL]:url:' \
                '--token[Runtime token]:token:' \
                '--name[Runtime name]:name:' \
                '--labels[Runtime labels (key=value,...)]:labels:' \
                '--pull[Pull mocks from mockd:// URI]:uri:' \
                '--cache[Local cache directory]:dir:_files -/' \
                '--graphql-schema[Path to GraphQL schema file]:file:_files' \
                '--graphql-path[GraphQL endpoint path]:path:' \
                '--grpc-port[gRPC server port]:port:' \
                '--grpc-proto[Path to .proto file]:file:_files' \
                '--grpc-reflection[Enable gRPC reflection]' \
                '--oauth-enabled[Enable OAuth provider]' \
                '--oauth-issuer[OAuth issuer URL]:url:' \
                '--oauth-port[OAuth server port]:port:' \
                '--mqtt-port[MQTT broker port]:port:' \
                '--mqtt-auth[Enable MQTT authentication]' \
                '--chaos-enabled[Enable chaos injection]' \
                '--chaos-latency[Add random latency]:range:' \
                '--chaos-error-rate[Error rate (0.0-1.0)]:rate:' \
                '--validate-spec[Path to OpenAPI spec]:file:_files' \
                '--validate-fail[Fail on validation error]' \
                '(-d --detach)'{-d,--detach}'[Run server in background]' \
                '--pid-file[Path to PID file]:file:_files'
            ;;
        start)
            _arguments \
                '(-p --port)'{-p,--port}'[HTTP server port]:port:' \
                '(-a --admin-port)'{-a,--admin-port}'[Admin API port]:port:' \
                '(-c --config)'{-c,--config}'[Path to mock configuration file]:file:_files' \
                '--https-port[HTTPS server port]:port:' \
                '--read-timeout[Read timeout in seconds]:seconds:' \
                '--write-timeout[Write timeout in seconds]:seconds:' \
                '--max-log-entries[Maximum request log entries]:count:' \
                '--auto-cert[Auto-generate TLS certificate]'
            ;;
        stop)
            _arguments \
                '1:component:(admin engine)' \
                '--pid-file[Path to PID file]:file:_files' \
                '(-f --force)'{-f,--force}'[Send SIGKILL instead of SIGTERM]' \
                '--timeout[Timeout in seconds]:seconds:'
            ;;
        status)
            _arguments \
                '--pid-file[Path to PID file]:file:_files' \
                '--json[Output in JSON format]'
            ;;
        init)
            _arguments \
                '(-o --output)'{-o,--output}'[Output filename]:file:_files' \
                '--force[Overwrite existing config file]' \
                '--format[Output format]:format:(yaml json)' \
                '(-i --interactive)'{-i,--interactive}'[Interactive mode]' \
                '(-t --template)'{-t,--template}'[Template name]:template:(default crud websocket-chat graphql-api grpc-service mqtt-iot list)'
            ;;
        new)
            _arguments \
                '(-t --template)'{-t,--template}'[Template name]:template:(blank crud auth pagination errors)' \
                '(-n --name)'{-n,--name}'[Collection name]:name:' \
                '(-o --output)'{-o,--output}'[Output file]:file:_files' \
                '--resource[Resource name]:resource:'
            ;;
        tunnel)
            if (( CURRENT == 3 )); then
                _values 'subcommand' 'status[Show tunnel status]' 'stop[Stop the tunnel]'
            else
                _arguments \
                    '(-p --port)'{-p,--port}'[HTTP server port]:port:' \
                    '--admin-port[Admin API port]:port:' \
                    '(-c --config)'{-c,--config}'[Path to mock configuration file]:file:_files' \
                    '--relay[Relay server URL]:url:' \
                    '--token[Authentication token]:token:' \
                    '(-s --subdomain)'{-s,--subdomain}'[Requested subdomain]:subdomain:' \
                    '--domain[Custom domain]:domain:' \
                    '--auth-token[Require token for incoming requests]:token:' \
                    '--auth-basic[Require Basic Auth (user:pass)]:credentials:' \
                    '--allow-ips[Allow only these IPs (CIDR list)]:ips:'
            fi
            ;;
        add)
            _arguments \
                '(-t --type)'{-t,--type}'[Mock type]:type:(http websocket graphql grpc mqtt soap)' \
                '(-m --method)'{-m,--method}'[HTTP method to match]:method:(GET POST PUT DELETE PATCH HEAD OPTIONS)' \
                '--path[URL path to match]:path:' \
                '(-s --status)'{-s,--status}'[Response status code]:code:' \
                '(-b --body)'{-b,--body}'[Response body]:body:' \
                '--body-file[Read response body from file]:file:_files' \
                '*'{-H,--header}'[Response header]:header:' \
                '*--match-header[Required request header]:header:' \
                '*--match-query[Required query param]:param:' \
                '(-n --name)'{-n,--name}'[Mock display name]:name:' \
                '--priority[Mock priority]:priority:' \
                '--delay[Response delay in milliseconds]:ms:' \
                '--message[WebSocket response message]:message:' \
                '--echo[Enable WebSocket echo mode]' \
                '--operation[GraphQL/SOAP operation name]:operation:' \
                '--op-type[GraphQL operation type]:type:(query mutation)' \
                '--response[Response data (JSON/XML)]:response:' \
                '--service[gRPC service name]:service:' \
                '--rpc-method[gRPC method name]:method:' \
                '--topic[MQTT topic pattern]:topic:' \
                '--payload[MQTT payload]:payload:' \
                '--qos[MQTT QoS level]:qos:(0 1 2)' \
                '--soap-action[SOAPAction header]:action:' \
                '--admin-url[Admin API base URL]:url:' \
                '--json[Output in JSON format]'
            ;;
        list)
            _arguments \
                '(-c --config)'{-c,--config}'[List mocks from config file]:file:_files' \
                '(-t --type)'{-t,--type}'[Filter by type]:type:(http websocket graphql grpc mqtt soap)' \
                '--admin-url[Admin API base URL]:url:' \
                '--json[Output in JSON format]'
            ;;
        get|delete)
            _arguments \
                '--admin-url[Admin API base URL]:url:' \
                '--json[Output in JSON format]'
            ;;
        import)
            _arguments \
                '--replace[Replace all existing mocks]' \
                '--admin-url[Admin API base URL]:url:' \
                ':file:_files -g "*.{json,yaml,yml}"'
            ;;
        export)
            _arguments \
                '(-o --output)'{-o,--output}'[Output file]:file:_files' \
                '(-n --name)'{-n,--name}'[Collection name]:name:' \
                '--admin-url[Admin API base URL]:url:'
            ;;
        logs)
            _arguments \
                '(-m --method)'{-m,--method}'[Filter by HTTP method]:method:(GET POST PUT DELETE PATCH HEAD OPTIONS)' \
                '(-p --path)'{-p,--path}'[Filter by path]:path:' \
                '--matched[Show only matched requests]' \
                '--unmatched[Show only unmatched requests]' \
                '(-n --limit)'{-n,--limit}'[Number of entries to show]:count:' \
                '--verbose[Show headers and body]' \
                '--clear[Clear all logs]' \
                '--admin-url[Admin API base URL]:url:' \
                '--json[Output in JSON format]'
            ;;
        config)
            _arguments '--json[Output in JSON format]'
            ;;
        proxy)
            if (( CURRENT == 3 )); then
                local -a proxy_commands
                proxy_commands=(
                    'start:Start the proxy server'
                    'stop:Stop the proxy server'
                    'status:Show proxy server status'
                    'mode:Get or set proxy mode'
                    'ca:Manage CA certificate'
                )
                _describe -t commands 'proxy commands' proxy_commands
            else
                case ${words[3]} in
                    start)
                        _arguments \
                            '(-p --port)'{-p,--port}'[Proxy server port]:port:' \
                            '(-m --mode)'{-m,--mode}'[Proxy mode]:mode:(record passthrough)' \
                            '(-s --session)'{-s,--session}'[Recording session name]:session:' \
                            '--ca-path[Path to CA certificate directory]:dir:_files -/' \
                            '--include[Path patterns to include]:patterns:' \
                            '--exclude[Path patterns to exclude]:patterns:' \
                            '--include-hosts[Host patterns to include]:hosts:' \
                            '--exclude-hosts[Host patterns to exclude]:hosts:'
                        ;;
                    mode)
                        _values 'mode' record passthrough
                        ;;
                    ca)
                        if (( CURRENT == 4 )); then
                            _values 'ca command' 'export[Export CA certificate]' 'generate[Generate new CA]'
                        else
                            _arguments \
                                '(-o --output)'{-o,--output}'[Output file]:file:_files' \
                                '--ca-path[CA certificate directory]:dir:_files -/'
                        fi
                        ;;
                esac
            fi
            ;;
        recordings)
            if (( CURRENT == 3 )); then
                local -a rec_commands
                rec_commands=(
                    'list:List all recordings'
                    'convert:Convert recordings to mocks'
                    'export:Export recordings to JSON'
                    'import:Import recordings from JSON'
                    'clear:Clear all recordings'
                )
                _describe -t commands 'recordings commands' rec_commands
            else
                case ${words[3]} in
                    list)
                        _arguments \
                            '--session[Filter by session ID]:session:' \
                            '--method[Filter by HTTP method]:method:(GET POST PUT DELETE PATCH HEAD OPTIONS)' \
                            '--path[Filter by request path]:path:' \
                            '--json[Output as JSON]' \
                            '--limit[Maximum recordings to show]:limit:'
                        ;;
                    convert)
                        _arguments \
                            '--session[Filter by session ID]:session:' \
                            '--deduplicate[Remove duplicate patterns]' \
                            '--include-headers[Include headers in matchers]' \
                            '(-o --output)'{-o,--output}'[Output file]:file:_files'
                        ;;
                    export)
                        _arguments \
                            '--session[Export specific session]:session:' \
                            '(-o --output)'{-o,--output}'[Output file]:file:_files'
                        ;;
                    import)
                        _arguments \
                            '(-i --input)'{-i,--input}'[Input file]:file:_files'
                        ;;
                    clear)
                        _arguments \
                            '(-f --force)'{-f,--force}'[Skip confirmation]'
                        ;;
                esac
            fi
            ;;
        convert)
            _arguments \
                '--recording[Convert single recording by ID]:id:' \
                '--session[Convert recordings from session]:session:' \
                '--path-filter[Glob pattern to filter paths]:pattern:' \
                '--method[HTTP methods to include]:methods:' \
                '--status[Status code filter]:filter:' \
                '--smart-match[Convert dynamic path segments]' \
                '--duplicates[Duplicate handling]:strategy:(first last all)' \
                '--include-headers[Include headers in matchers]' \
                '--check-sensitive[Check for sensitive data]' \
                '(-o --output)'{-o,--output}'[Output file]:file:_files'
            ;;
        generate)
            _arguments \
                '(-i --input)'{-i,--input}'[Input OpenAPI spec file]:file:_files' \
                '(-p --prompt)'{-p,--prompt}'[Natural language description]:prompt:' \
                '(-o --output)'{-o,--output}'[Output file]:file:_files' \
                '--ai[Enable AI-powered generation]' \
                '--provider[AI provider]:provider:(openai anthropic ollama)' \
                '--model[AI model to use]:model:' \
                '--dry-run[Preview without saving]' \
                '--admin-url[Admin API base URL]:url:'
            ;;
        enhance)
            _arguments \
                '--ai[Enable AI-powered enhancement]' \
                '--provider[AI provider]:provider:(openai anthropic ollama)' \
                '--model[AI model to use]:model:' \
                '--admin-url[Admin API base URL]:url:'
            ;;
        stream-recordings)
            if (( CURRENT == 3 )); then
                local -a stream_commands
                stream_commands=(
                    'list:List all stream recordings'
                    'show:Show details of a recording'
                    'delete:Delete a recording'
                    'export:Export recording to JSON'
                    'convert:Convert recording to mock config'
                    'stats:Show storage statistics'
                    'vacuum:Remove soft-deleted recordings'
                    'sessions:List active recording sessions'
                )
                _describe -t commands 'stream-recordings commands' stream_commands
            else
                case ${words[3]} in
                    list|ls)
                        _arguments \
                            '--protocol[Filter by protocol]:protocol:(websocket sse)' \
                            '--path[Filter by path prefix]:path:' \
                            '--status[Filter by status]:status:(complete incomplete recording)' \
                            '--json[Output as JSON]' \
                            '--limit[Maximum recordings]:limit:' \
                            '--offset[Offset for pagination]:offset:' \
                            '--sort[Sort by field]:field:(startTime name size)' \
                            '--order[Sort order]:order:(asc desc)' \
                            '--include-deleted[Include soft-deleted]'
                        ;;
                    show|get)
                        _arguments '--json[Output as JSON]'
                        ;;
                    delete|rm)
                        _arguments \
                            '(-f --force)'{-f,--force}'[Skip confirmation]' \
                            '--permanent[Permanently delete]'
                        ;;
                    export)
                        _arguments \
                            '(-o --output)'{-o,--output}'[Output file]:file:_files'
                        ;;
                    convert)
                        _arguments \
                            '(-o --output)'{-o,--output}'[Output file]:file:_files' \
                            '--simplify-timing[Normalize timing]' \
                            '--min-delay[Minimum delay (ms)]:ms:' \
                            '--max-delay[Maximum delay (ms)]:ms:' \
                            '--include-client[Include client messages]' \
                            '--deduplicate[Remove duplicate messages]'
                        ;;
                    stats|sessions)
                        _arguments '--json[Output as JSON]'
                        ;;
                    vacuum)
                        _arguments '(-f --force)'{-f,--force}'[Skip confirmation]'
                        ;;
                esac
            fi
            ;;
        graphql)
            if (( CURRENT == 3 )); then
                local -a gql_commands
                gql_commands=(
                    'validate:Validate a GraphQL schema'
                    'query:Execute a GraphQL query'
                )
                _describe -t commands 'graphql commands' gql_commands
            else
                case ${words[3]} in
                    query)
                        _arguments \
                            '(-v --variables)'{-v,--variables}'[JSON variables]:json:' \
                            '(-o --operation)'{-o,--operation}'[Operation name]:name:' \
                            '(-H --header)'{-H,--header}'[Additional headers]:headers:' \
                            '--pretty[Pretty print output]'
                        ;;
                esac
            fi
            ;;
        chaos)
            if (( CURRENT == 3 )); then
                local -a chaos_commands
                chaos_commands=(
                    'enable:Enable chaos injection'
                    'disable:Disable chaos injection'
                    'status:Show chaos configuration'
                )
                _describe -t commands 'chaos commands' chaos_commands
            else
                case ${words[3]} in
                    enable)
                        _arguments \
                            '--admin-url[Admin API base URL]:url:' \
                            '(-l --latency)'{-l,--latency}'[Add random latency]:range:' \
                            '(-e --error-rate)'{-e,--error-rate}'[Error rate (0.0-1.0)]:rate:' \
                            '--error-code[HTTP error code]:code:' \
                            '(-p --path)'{-p,--path}'[Path pattern (regex)]:pattern:' \
                            '--probability[Probability (0.0-1.0)]:prob:'
                        ;;
                    disable|status)
                        _arguments \
                            '--admin-url[Admin API base URL]:url:' \
                            '--json[Output in JSON format]'
                        ;;
                esac
            fi
            ;;
        grpc)
            if (( CURRENT == 3 )); then
                local -a grpc_commands
                grpc_commands=(
                    'list:List services and methods'
                    'call:Call a gRPC method'
                )
                _describe -t commands 'grpc commands' grpc_commands
            else
                case ${words[3]} in
                    list)
                        _arguments \
                            '(-I --import)'{-I,--import}'[Import path for proto includes]:path:_files -/'
                        ;;
                    call)
                        _arguments \
                            '(-m --metadata)'{-m,--metadata}'[gRPC metadata]:metadata:' \
                            '--plaintext[Use plaintext (no TLS)]' \
                            '--pretty[Pretty print output]'
                        ;;
                esac
            fi
            ;;
        mqtt)
            if (( CURRENT == 3 )); then
                local -a mqtt_commands
                mqtt_commands=(
                    'publish:Publish a message'
                    'subscribe:Subscribe to a topic'
                    'status:Show MQTT broker status'
                )
                _describe -t commands 'mqtt commands' mqtt_commands
            else
                case ${words[3]} in
                    publish)
                        _arguments \
                            '(-b --broker)'{-b,--broker}'[MQTT broker address]:address:' \
                            '(-u --username)'{-u,--username}'[MQTT username]:username:' \
                            '(-P --password)'{-P,--password}'[MQTT password]:password:' \
                            '--qos[QoS level]:qos:(0 1 2)' \
                            '--retain[Retain message]'
                        ;;
                    subscribe)
                        _arguments \
                            '(-b --broker)'{-b,--broker}'[MQTT broker address]:address:' \
                            '(-u --username)'{-u,--username}'[MQTT username]:username:' \
                            '(-P --password)'{-P,--password}'[MQTT password]:password:' \
                            '--qos[QoS level]:qos:(0 1 2)' \
                            '(-n --count)'{-n,--count}'[Number of messages]:count:' \
                            '(-t --timeout)'{-t,--timeout}'[Timeout duration]:duration:'
                        ;;
                    status)
                        _arguments \
                            '--admin-url[Admin API base URL]:url:' \
                            '--json[Output in JSON format]'
                        ;;
                esac
            fi
            ;;
        websocket)
            if (( CURRENT == 3 )); then
                local -a ws_commands
                ws_commands=(
                    'connect:Interactive WebSocket client (REPL mode)'
                    'send:Send a single message and exit'
                    'listen:Stream incoming messages'
                    'status:Show WebSocket handler status'
                )
                _describe -t commands 'websocket commands' ws_commands
            else
                case ${words[3]} in
                    connect)
                        _arguments \
                            '*'{-H,--header}'[Custom headers (key:value)]:header:' \
                            '--subprotocol[WebSocket subprotocol]:subprotocol:' \
                            '(-t --timeout)'{-t,--timeout}'[Connection timeout]:timeout:' \
                            '--json[Output messages in JSON format]'
                        ;;
                    send)
                        _arguments \
                            '*'{-H,--header}'[Custom headers (key:value)]:header:' \
                            '--subprotocol[WebSocket subprotocol]:subprotocol:' \
                            '(-t --timeout)'{-t,--timeout}'[Connection timeout]:timeout:' \
                            '--json[Output result in JSON format]'
                        ;;
                    listen)
                        _arguments \
                            '*'{-H,--header}'[Custom headers (key:value)]:header:' \
                            '--subprotocol[WebSocket subprotocol]:subprotocol:' \
                            '(-t --timeout)'{-t,--timeout}'[Connection timeout]:timeout:' \
                            '(-n --count)'{-n,--count}'[Number of messages to receive]:count:' \
                            '--json[Output messages in JSON format]'
                        ;;
                    status)
                        _arguments \
                            '--admin-url[Admin API base URL]:url:' \
                            '--json[Output in JSON format]'
                        ;;
                esac
            fi
            ;;
        soap)
            if (( CURRENT == 3 )); then
                local -a soap_commands
                soap_commands=(
                    'validate:Validate a WSDL file'
                    'call:Call a SOAP operation'
                )
                _describe -t commands 'soap commands' soap_commands
            else
                case ${words[3]} in
                    call)
                        _arguments \
                            '(-a --action)'{-a,--action}'[SOAPAction header]:action:' \
                            '(-b --body)'{-b,--body}'[SOAP body content]:body:' \
                            '(-H --header)'{-H,--header}'[Additional headers]:headers:' \
                            '--soap12[Use SOAP 1.2]' \
                            '--pretty[Pretty print output]' \
                            '--timeout[Request timeout]:seconds:'
                        ;;
                esac
            fi
            ;;
        templates)
            if (( CURRENT == 3 )); then
                local -a tmpl_commands
                tmpl_commands=(
                    'list:List available templates'
                    'add:Download and import a template'
                )
                _describe -t commands 'templates commands' tmpl_commands
            else
                case ${words[3]} in
                    list)
                        _arguments \
                            '(-c --category)'{-c,--category}'[Filter by category]:category:(protocols services patterns)' \
                            '--base-url[Templates repository URL]:url:'
                        ;;
                    add)
                        _arguments \
                            '(-o --output)'{-o,--output}'[Save to file]:file:_files' \
                            '--admin-url[Admin API base URL]:url:' \
                            '--base-url[Templates repository URL]:url:' \
                            '--dry-run[Preview without importing]'
                        ;;
                esac
            fi
            ;;
        completion)
            _values 'shell' bash zsh fish
            ;;
        version)
            _arguments '--json[Output in JSON format]'
            ;;
        help)
            _values 'topic' config matching templating formats websocket graphql grpc mqtt soap sse
            ;;
        doctor)
            _arguments \
                '(-c --config)'{-c,--config}'[Path to config file to validate]:file:_files' \
                '(-p --port)'{-p,--port}'[Mock server port to check]:port:' \
                '(-a --admin-port)'{-a,--admin-port}'[Admin API port to check]:port:'
            ;;
    esac
}

_mockd
`

const fishCompletion = `# mockd fish completion

# Disable file completions for base command
complete -c mockd -f

# Main commands
complete -c mockd -n '__fish_use_subcommand' -a 'serve' -d 'Start the mock server (default)'
complete -c mockd -n '__fish_use_subcommand' -a 'start' -d 'Start the mock server (alias)'
complete -c mockd -n '__fish_use_subcommand' -a 'stop' -d 'Stop a running mockd server'
complete -c mockd -n '__fish_use_subcommand' -a 'status' -d 'Show status of running server'
complete -c mockd -n '__fish_use_subcommand' -a 'init' -d 'Create a starter config file'
complete -c mockd -n '__fish_use_subcommand' -a 'new' -d 'Create mocks from templates'
complete -c mockd -n '__fish_use_subcommand' -a 'add' -d 'Add a new mock endpoint'
complete -c mockd -n '__fish_use_subcommand' -a 'list' -d 'List all configured mocks'
complete -c mockd -n '__fish_use_subcommand' -a 'get' -d 'Get details of a specific mock'
complete -c mockd -n '__fish_use_subcommand' -a 'delete' -d 'Delete a mock by ID'
complete -c mockd -n '__fish_use_subcommand' -a 'import' -d 'Import mocks from a configuration file'
complete -c mockd -n '__fish_use_subcommand' -a 'export' -d 'Export current mocks to stdout or file'
complete -c mockd -n '__fish_use_subcommand' -a 'logs' -d 'View request logs'
complete -c mockd -n '__fish_use_subcommand' -a 'config' -d 'Show effective configuration'
complete -c mockd -n '__fish_use_subcommand' -a 'proxy' -d 'Manage the MITM proxy'
complete -c mockd -n '__fish_use_subcommand' -a 'recordings' -d 'Manage recorded API traffic'
complete -c mockd -n '__fish_use_subcommand' -a 'convert' -d 'Convert recordings to mock definitions'
complete -c mockd -n '__fish_use_subcommand' -a 'generate' -d 'Generate mocks from OpenAPI or AI'
complete -c mockd -n '__fish_use_subcommand' -a 'enhance' -d 'Enhance mocks with AI-generated data'
complete -c mockd -n '__fish_use_subcommand' -a 'stream-recordings' -d 'Manage WebSocket/SSE recordings'
complete -c mockd -n '__fish_use_subcommand' -a 'graphql' -d 'Validate schemas and execute queries'
complete -c mockd -n '__fish_use_subcommand' -a 'chaos' -d 'Manage chaos injection'
complete -c mockd -n '__fish_use_subcommand' -a 'grpc' -d 'Manage and test gRPC endpoints'
complete -c mockd -n '__fish_use_subcommand' -a 'mqtt' -d 'Publish, subscribe, and manage MQTT'
complete -c mockd -n '__fish_use_subcommand' -a 'websocket' -d 'Connect, send, and listen to WebSocket endpoints'
complete -c mockd -n '__fish_use_subcommand' -a 'soap' -d 'Validate WSDL and call SOAP operations'
complete -c mockd -n '__fish_use_subcommand' -a 'templates' -d 'List and add templates from library'
complete -c mockd -n '__fish_use_subcommand' -a 'tunnel' -d 'Expose local mocks via cloud relay'
complete -c mockd -n '__fish_use_subcommand' -a 'completion' -d 'Generate shell completion scripts'
complete -c mockd -n '__fish_use_subcommand' -a 'version' -d 'Show version information'
complete -c mockd -n '__fish_use_subcommand' -a 'help' -d 'Show help for topics'
complete -c mockd -n '__fish_use_subcommand' -a 'doctor' -d 'Diagnose common setup issues'

# serve options
complete -c mockd -n '__fish_seen_subcommand_from serve' -s p -l port -d 'HTTP server port'
complete -c mockd -n '__fish_seen_subcommand_from serve' -s a -l admin-port -d 'Admin API port'
complete -c mockd -n '__fish_seen_subcommand_from serve' -s c -l config -d 'Path to mock configuration file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from serve' -l https-port -d 'HTTPS server port'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l read-timeout -d 'Read timeout in seconds'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l write-timeout -d 'Write timeout in seconds'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l max-log-entries -d 'Maximum request log entries'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l auto-cert -d 'Auto-generate TLS certificate'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l tls-cert -d 'Path to TLS certificate file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from serve' -l tls-key -d 'Path to TLS private key file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from serve' -l tls-auto -d 'Auto-generate self-signed certificate'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l mtls-enabled -d 'Enable mTLS client certificate validation'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l mtls-client-auth -d 'Client auth mode' -a 'none request require verify-if-given require-and-verify'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l mtls-ca -d 'Path to CA certificate' -r -F
complete -c mockd -n '__fish_seen_subcommand_from serve' -l mtls-allowed-cns -d 'Comma-separated allowed Common Names'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l audit-enabled -d 'Enable audit logging'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l audit-file -d 'Path to audit log file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from serve' -l audit-level -d 'Log level' -a 'debug info warn error'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l register -d 'Register with control plane as runtime'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l control-plane -d 'Control plane URL'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l token -d 'Runtime token'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l name -d 'Runtime name'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l labels -d 'Runtime labels (key=value,...)'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l pull -d 'Pull mocks from mockd:// URI'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l cache -d 'Local cache directory' -r -F
complete -c mockd -n '__fish_seen_subcommand_from serve' -l graphql-schema -d 'Path to GraphQL schema file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from serve' -l graphql-path -d 'GraphQL endpoint path'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l grpc-port -d 'gRPC server port'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l grpc-proto -d 'Path to .proto file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from serve' -l grpc-reflection -d 'Enable gRPC reflection'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l oauth-enabled -d 'Enable OAuth provider'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l oauth-issuer -d 'OAuth issuer URL'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l oauth-port -d 'OAuth server port'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l mqtt-port -d 'MQTT broker port'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l mqtt-auth -d 'Enable MQTT authentication'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l chaos-enabled -d 'Enable chaos injection'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l chaos-latency -d 'Add random latency (e.g., 10ms-100ms)'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l chaos-error-rate -d 'Error rate (0.0-1.0)'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l validate-spec -d 'Path to OpenAPI spec' -r -F
complete -c mockd -n '__fish_seen_subcommand_from serve' -l validate-fail -d 'Fail on validation error'
complete -c mockd -n '__fish_seen_subcommand_from serve' -s d -l detach -d 'Run server in background'
complete -c mockd -n '__fish_seen_subcommand_from serve' -l pid-file -d 'Path to PID file' -r -F

# start options
complete -c mockd -n '__fish_seen_subcommand_from start' -s p -l port -d 'HTTP server port'
complete -c mockd -n '__fish_seen_subcommand_from start' -s a -l admin-port -d 'Admin API port'
complete -c mockd -n '__fish_seen_subcommand_from start' -s c -l config -d 'Path to mock configuration file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from start' -l https-port -d 'HTTPS server port'
complete -c mockd -n '__fish_seen_subcommand_from start' -l read-timeout -d 'Read timeout in seconds'
complete -c mockd -n '__fish_seen_subcommand_from start' -l write-timeout -d 'Write timeout in seconds'
complete -c mockd -n '__fish_seen_subcommand_from start' -l max-log-entries -d 'Maximum request log entries'
complete -c mockd -n '__fish_seen_subcommand_from start' -l auto-cert -d 'Auto-generate TLS certificate'

# stop options
complete -c mockd -n '__fish_seen_subcommand_from stop' -a 'admin' -d 'Stop admin component'
complete -c mockd -n '__fish_seen_subcommand_from stop' -a 'engine' -d 'Stop engine component'
complete -c mockd -n '__fish_seen_subcommand_from stop' -l pid-file -d 'Path to PID file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from stop' -s f -l force -d 'Send SIGKILL instead of SIGTERM'
complete -c mockd -n '__fish_seen_subcommand_from stop' -l timeout -d 'Timeout in seconds'

# status options
complete -c mockd -n '__fish_seen_subcommand_from status' -l pid-file -d 'Path to PID file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from status' -l json -d 'Output in JSON format'

# init options
complete -c mockd -n '__fish_seen_subcommand_from init' -s o -l output -d 'Output filename' -r -F
complete -c mockd -n '__fish_seen_subcommand_from init' -l force -d 'Overwrite existing config file'
complete -c mockd -n '__fish_seen_subcommand_from init' -l format -d 'Output format' -a 'yaml json'
complete -c mockd -n '__fish_seen_subcommand_from init' -s i -l interactive -d 'Interactive mode'
complete -c mockd -n '__fish_seen_subcommand_from init' -s t -l template -d 'Template name' -a 'default crud websocket-chat graphql-api grpc-service mqtt-iot list'

# doctor options
complete -c mockd -n '__fish_seen_subcommand_from doctor' -s c -l config -d 'Path to config file to validate' -r -F
complete -c mockd -n '__fish_seen_subcommand_from doctor' -s p -l port -d 'Mock server port to check'
complete -c mockd -n '__fish_seen_subcommand_from doctor' -s a -l admin-port -d 'Admin API port to check'

# new options
complete -c mockd -n '__fish_seen_subcommand_from new' -s t -l template -d 'Template name' -a 'blank crud auth pagination errors'
complete -c mockd -n '__fish_seen_subcommand_from new' -s n -l name -d 'Collection name'
complete -c mockd -n '__fish_seen_subcommand_from new' -s o -l output -d 'Output file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from new' -l resource -d 'Resource name'

# tunnel options
complete -c mockd -n '__fish_seen_subcommand_from tunnel' -a 'status' -d 'Show tunnel status'
complete -c mockd -n '__fish_seen_subcommand_from tunnel' -a 'stop' -d 'Stop the tunnel'
complete -c mockd -n '__fish_seen_subcommand_from tunnel' -s p -l port -d 'HTTP server port'
complete -c mockd -n '__fish_seen_subcommand_from tunnel' -l admin-port -d 'Admin API port'
complete -c mockd -n '__fish_seen_subcommand_from tunnel' -s c -l config -d 'Path to mock configuration file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from tunnel' -l relay -d 'Relay server URL'
complete -c mockd -n '__fish_seen_subcommand_from tunnel' -l token -d 'Authentication token'
complete -c mockd -n '__fish_seen_subcommand_from tunnel' -s s -l subdomain -d 'Requested subdomain'
complete -c mockd -n '__fish_seen_subcommand_from tunnel' -l domain -d 'Custom domain'
complete -c mockd -n '__fish_seen_subcommand_from tunnel' -l auth-token -d 'Require token for incoming requests'
complete -c mockd -n '__fish_seen_subcommand_from tunnel' -l auth-basic -d 'Require Basic Auth (user:pass)'
complete -c mockd -n '__fish_seen_subcommand_from tunnel' -l allow-ips -d 'Allow only these IPs (CIDR list)'

# add options
complete -c mockd -n '__fish_seen_subcommand_from add' -s t -l type -d 'Mock type' -a 'http websocket graphql grpc mqtt soap'
complete -c mockd -n '__fish_seen_subcommand_from add' -s m -l method -d 'HTTP method to match' -a 'GET POST PUT DELETE PATCH HEAD OPTIONS'
complete -c mockd -n '__fish_seen_subcommand_from add' -l path -d 'URL path to match'
complete -c mockd -n '__fish_seen_subcommand_from add' -s s -l status -d 'Response status code'
complete -c mockd -n '__fish_seen_subcommand_from add' -s b -l body -d 'Response body'
complete -c mockd -n '__fish_seen_subcommand_from add' -l body-file -d 'Read response body from file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from add' -s H -l header -d 'Response header (key:value)'
complete -c mockd -n '__fish_seen_subcommand_from add' -l match-header -d 'Required request header'
complete -c mockd -n '__fish_seen_subcommand_from add' -l match-query -d 'Required query param'
complete -c mockd -n '__fish_seen_subcommand_from add' -s n -l name -d 'Mock display name'
complete -c mockd -n '__fish_seen_subcommand_from add' -l priority -d 'Mock priority'
complete -c mockd -n '__fish_seen_subcommand_from add' -l delay -d 'Response delay in milliseconds'
complete -c mockd -n '__fish_seen_subcommand_from add' -l message -d 'WebSocket response message'
complete -c mockd -n '__fish_seen_subcommand_from add' -l echo -d 'Enable WebSocket echo mode'
complete -c mockd -n '__fish_seen_subcommand_from add' -l operation -d 'GraphQL/SOAP operation name'
complete -c mockd -n '__fish_seen_subcommand_from add' -l op-type -d 'GraphQL operation type' -a 'query mutation'
complete -c mockd -n '__fish_seen_subcommand_from add' -l response -d 'Response data (JSON/XML)'
complete -c mockd -n '__fish_seen_subcommand_from add' -l service -d 'gRPC service name'
complete -c mockd -n '__fish_seen_subcommand_from add' -l rpc-method -d 'gRPC method name'
complete -c mockd -n '__fish_seen_subcommand_from add' -l topic -d 'MQTT topic pattern'
complete -c mockd -n '__fish_seen_subcommand_from add' -l payload -d 'MQTT payload'
complete -c mockd -n '__fish_seen_subcommand_from add' -l qos -d 'MQTT QoS level' -a '0 1 2'
complete -c mockd -n '__fish_seen_subcommand_from add' -l soap-action -d 'SOAPAction header'
complete -c mockd -n '__fish_seen_subcommand_from add' -l admin-url -d 'Admin API base URL'
complete -c mockd -n '__fish_seen_subcommand_from add' -l json -d 'Output in JSON format'

# list options
complete -c mockd -n '__fish_seen_subcommand_from list' -s c -l config -d 'List mocks from config file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from list' -s t -l type -d 'Filter by type' -a 'http websocket graphql grpc mqtt soap'
complete -c mockd -n '__fish_seen_subcommand_from list' -l admin-url -d 'Admin API base URL'
complete -c mockd -n '__fish_seen_subcommand_from list' -l json -d 'Output in JSON format'

# get/delete options
complete -c mockd -n '__fish_seen_subcommand_from get delete' -l admin-url -d 'Admin API base URL'
complete -c mockd -n '__fish_seen_subcommand_from get' -l json -d 'Output in JSON format'

# import options
complete -c mockd -n '__fish_seen_subcommand_from import' -l replace -d 'Replace all existing mocks'
complete -c mockd -n '__fish_seen_subcommand_from import' -l admin-url -d 'Admin API base URL'

# export options
complete -c mockd -n '__fish_seen_subcommand_from export' -s o -l output -d 'Output file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from export' -s n -l name -d 'Collection name'
complete -c mockd -n '__fish_seen_subcommand_from export' -l admin-url -d 'Admin API base URL'

# logs options
complete -c mockd -n '__fish_seen_subcommand_from logs' -s m -l method -d 'Filter by HTTP method' -a 'GET POST PUT DELETE PATCH HEAD OPTIONS'
complete -c mockd -n '__fish_seen_subcommand_from logs' -s p -l path -d 'Filter by path'
complete -c mockd -n '__fish_seen_subcommand_from logs' -l matched -d 'Show only matched requests'
complete -c mockd -n '__fish_seen_subcommand_from logs' -l unmatched -d 'Show only unmatched requests'
complete -c mockd -n '__fish_seen_subcommand_from logs' -s n -l limit -d 'Number of entries to show'
complete -c mockd -n '__fish_seen_subcommand_from logs' -l verbose -d 'Show headers and body'
complete -c mockd -n '__fish_seen_subcommand_from logs' -l clear -d 'Clear all logs'
complete -c mockd -n '__fish_seen_subcommand_from logs' -l admin-url -d 'Admin API base URL'
complete -c mockd -n '__fish_seen_subcommand_from logs' -l json -d 'Output in JSON format'

# config options
complete -c mockd -n '__fish_seen_subcommand_from config' -l json -d 'Output in JSON format'

# proxy subcommands and options
complete -c mockd -n '__fish_seen_subcommand_from proxy; and not __fish_seen_subcommand_from start stop status mode ca' -a 'start' -d 'Start the proxy server'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and not __fish_seen_subcommand_from start stop status mode ca' -a 'stop' -d 'Stop the proxy server'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and not __fish_seen_subcommand_from start stop status mode ca' -a 'status' -d 'Show proxy server status'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and not __fish_seen_subcommand_from start stop status mode ca' -a 'mode' -d 'Get or set proxy mode'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and not __fish_seen_subcommand_from start stop status mode ca' -a 'ca' -d 'Manage CA certificate'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and __fish_seen_subcommand_from start' -s p -l port -d 'Proxy server port'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and __fish_seen_subcommand_from start' -s m -l mode -d 'Proxy mode' -a 'record passthrough'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and __fish_seen_subcommand_from start' -s s -l session -d 'Recording session name'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and __fish_seen_subcommand_from start' -l ca-path -d 'Path to CA certificate directory' -r -F
complete -c mockd -n '__fish_seen_subcommand_from proxy; and __fish_seen_subcommand_from start' -l include -d 'Path patterns to include'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and __fish_seen_subcommand_from start' -l exclude -d 'Path patterns to exclude'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and __fish_seen_subcommand_from start' -l include-hosts -d 'Host patterns to include'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and __fish_seen_subcommand_from start' -l exclude-hosts -d 'Host patterns to exclude'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and __fish_seen_subcommand_from mode' -a 'record passthrough'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and __fish_seen_subcommand_from ca' -a 'export' -d 'Export CA certificate'
complete -c mockd -n '__fish_seen_subcommand_from proxy; and __fish_seen_subcommand_from ca' -a 'generate' -d 'Generate new CA'

# recordings subcommands and options
complete -c mockd -n '__fish_seen_subcommand_from recordings; and not __fish_seen_subcommand_from list convert export import clear' -a 'list' -d 'List all recordings'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and not __fish_seen_subcommand_from list convert export import clear' -a 'convert' -d 'Convert recordings to mocks'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and not __fish_seen_subcommand_from list convert export import clear' -a 'export' -d 'Export recordings to JSON'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and not __fish_seen_subcommand_from list convert export import clear' -a 'import' -d 'Import recordings from JSON'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and not __fish_seen_subcommand_from list convert export import clear' -a 'clear' -d 'Clear all recordings'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and __fish_seen_subcommand_from list' -l session -d 'Filter by session ID'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and __fish_seen_subcommand_from list' -l method -d 'Filter by HTTP method' -a 'GET POST PUT DELETE PATCH HEAD OPTIONS'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and __fish_seen_subcommand_from list' -l path -d 'Filter by request path'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and __fish_seen_subcommand_from list' -l json -d 'Output as JSON'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and __fish_seen_subcommand_from list' -l limit -d 'Maximum recordings to show'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and __fish_seen_subcommand_from convert' -l session -d 'Filter by session ID'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and __fish_seen_subcommand_from convert' -l deduplicate -d 'Remove duplicate patterns'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and __fish_seen_subcommand_from convert' -l include-headers -d 'Include headers in matchers'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and __fish_seen_subcommand_from convert' -s o -l output -d 'Output file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from recordings; and __fish_seen_subcommand_from export' -l session -d 'Export specific session'
complete -c mockd -n '__fish_seen_subcommand_from recordings; and __fish_seen_subcommand_from export' -s o -l output -d 'Output file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from recordings; and __fish_seen_subcommand_from import' -s i -l input -d 'Input file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from recordings; and __fish_seen_subcommand_from clear' -s f -l force -d 'Skip confirmation'

# convert options
complete -c mockd -n '__fish_seen_subcommand_from convert' -l recording -d 'Convert single recording by ID'
complete -c mockd -n '__fish_seen_subcommand_from convert' -l session -d 'Convert recordings from session'
complete -c mockd -n '__fish_seen_subcommand_from convert' -l path-filter -d 'Glob pattern to filter paths'
complete -c mockd -n '__fish_seen_subcommand_from convert' -l method -d 'HTTP methods to include'
complete -c mockd -n '__fish_seen_subcommand_from convert' -l status -d 'Status code filter'
complete -c mockd -n '__fish_seen_subcommand_from convert' -l smart-match -d 'Convert dynamic path segments'
complete -c mockd -n '__fish_seen_subcommand_from convert' -l duplicates -d 'Duplicate handling' -a 'first last all'
complete -c mockd -n '__fish_seen_subcommand_from convert' -l include-headers -d 'Include headers in matchers'
complete -c mockd -n '__fish_seen_subcommand_from convert' -l check-sensitive -d 'Check for sensitive data'
complete -c mockd -n '__fish_seen_subcommand_from convert' -s o -l output -d 'Output file' -r -F

# generate options
complete -c mockd -n '__fish_seen_subcommand_from generate' -s i -l input -d 'Input OpenAPI spec file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from generate' -s p -l prompt -d 'Natural language description'
complete -c mockd -n '__fish_seen_subcommand_from generate' -s o -l output -d 'Output file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from generate' -l ai -d 'Enable AI-powered generation'
complete -c mockd -n '__fish_seen_subcommand_from generate' -l provider -d 'AI provider' -a 'openai anthropic ollama'
complete -c mockd -n '__fish_seen_subcommand_from generate' -l model -d 'AI model to use'
complete -c mockd -n '__fish_seen_subcommand_from generate' -l dry-run -d 'Preview without saving'
complete -c mockd -n '__fish_seen_subcommand_from generate' -l admin-url -d 'Admin API base URL'

# enhance options
complete -c mockd -n '__fish_seen_subcommand_from enhance' -l ai -d 'Enable AI-powered enhancement'
complete -c mockd -n '__fish_seen_subcommand_from enhance' -l provider -d 'AI provider' -a 'openai anthropic ollama'
complete -c mockd -n '__fish_seen_subcommand_from enhance' -l model -d 'AI model to use'
complete -c mockd -n '__fish_seen_subcommand_from enhance' -l admin-url -d 'Admin API base URL'

# stream-recordings subcommands and options
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and not __fish_seen_subcommand_from list show delete export convert stats vacuum sessions' -a 'list' -d 'List all stream recordings'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and not __fish_seen_subcommand_from list show delete export convert stats vacuum sessions' -a 'show' -d 'Show details of a recording'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and not __fish_seen_subcommand_from list show delete export convert stats vacuum sessions' -a 'delete' -d 'Delete a recording'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and not __fish_seen_subcommand_from list show delete export convert stats vacuum sessions' -a 'export' -d 'Export recording to JSON'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and not __fish_seen_subcommand_from list show delete export convert stats vacuum sessions' -a 'convert' -d 'Convert recording to mock config'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and not __fish_seen_subcommand_from list show delete export convert stats vacuum sessions' -a 'stats' -d 'Show storage statistics'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and not __fish_seen_subcommand_from list show delete export convert stats vacuum sessions' -a 'vacuum' -d 'Remove soft-deleted recordings'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and not __fish_seen_subcommand_from list show delete export convert stats vacuum sessions' -a 'sessions' -d 'List active recording sessions'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from list' -l protocol -d 'Filter by protocol' -a 'websocket sse'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from list' -l path -d 'Filter by path prefix'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from list' -l status -d 'Filter by status' -a 'complete incomplete recording'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from list' -l json -d 'Output as JSON'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from list' -l limit -d 'Maximum recordings'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from list' -l offset -d 'Offset for pagination'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from list' -l sort -d 'Sort by field' -a 'startTime name size'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from list' -l order -d 'Sort order' -a 'asc desc'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from list' -l include-deleted -d 'Include soft-deleted'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from show' -l json -d 'Output as JSON'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from delete' -s f -l force -d 'Skip confirmation'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from delete' -l permanent -d 'Permanently delete'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from export' -s o -l output -d 'Output file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from convert' -s o -l output -d 'Output file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from convert' -l simplify-timing -d 'Normalize timing'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from convert' -l min-delay -d 'Minimum delay (ms)'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from convert' -l max-delay -d 'Maximum delay (ms)'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from convert' -l include-client -d 'Include client messages'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from convert' -l deduplicate -d 'Remove duplicate messages'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from stats' -l json -d 'Output as JSON'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from sessions' -l json -d 'Output as JSON'
complete -c mockd -n '__fish_seen_subcommand_from stream-recordings; and __fish_seen_subcommand_from vacuum' -s f -l force -d 'Skip confirmation'

# graphql subcommands and options
complete -c mockd -n '__fish_seen_subcommand_from graphql; and not __fish_seen_subcommand_from validate query' -a 'validate' -d 'Validate a GraphQL schema'
complete -c mockd -n '__fish_seen_subcommand_from graphql; and not __fish_seen_subcommand_from validate query' -a 'query' -d 'Execute a GraphQL query'
complete -c mockd -n '__fish_seen_subcommand_from graphql; and __fish_seen_subcommand_from query' -s v -l variables -d 'JSON variables'
complete -c mockd -n '__fish_seen_subcommand_from graphql; and __fish_seen_subcommand_from query' -s o -l operation -d 'Operation name'
complete -c mockd -n '__fish_seen_subcommand_from graphql; and __fish_seen_subcommand_from query' -s H -l header -d 'Additional headers'
complete -c mockd -n '__fish_seen_subcommand_from graphql; and __fish_seen_subcommand_from query' -l pretty -d 'Pretty print output'

# chaos subcommands and options
complete -c mockd -n '__fish_seen_subcommand_from chaos; and not __fish_seen_subcommand_from enable disable status' -a 'enable' -d 'Enable chaos injection'
complete -c mockd -n '__fish_seen_subcommand_from chaos; and not __fish_seen_subcommand_from enable disable status' -a 'disable' -d 'Disable chaos injection'
complete -c mockd -n '__fish_seen_subcommand_from chaos; and not __fish_seen_subcommand_from enable disable status' -a 'status' -d 'Show chaos configuration'
complete -c mockd -n '__fish_seen_subcommand_from chaos; and __fish_seen_subcommand_from enable' -l admin-url -d 'Admin API base URL'
complete -c mockd -n '__fish_seen_subcommand_from chaos; and __fish_seen_subcommand_from enable' -s l -l latency -d 'Add random latency'
complete -c mockd -n '__fish_seen_subcommand_from chaos; and __fish_seen_subcommand_from enable' -s e -l error-rate -d 'Error rate (0.0-1.0)'
complete -c mockd -n '__fish_seen_subcommand_from chaos; and __fish_seen_subcommand_from enable' -l error-code -d 'HTTP error code'
complete -c mockd -n '__fish_seen_subcommand_from chaos; and __fish_seen_subcommand_from enable' -s p -l path -d 'Path pattern (regex)'
complete -c mockd -n '__fish_seen_subcommand_from chaos; and __fish_seen_subcommand_from enable' -l probability -d 'Probability (0.0-1.0)'
complete -c mockd -n '__fish_seen_subcommand_from chaos; and __fish_seen_subcommand_from disable' -l admin-url -d 'Admin API base URL'
complete -c mockd -n '__fish_seen_subcommand_from chaos; and __fish_seen_subcommand_from status' -l admin-url -d 'Admin API base URL'
complete -c mockd -n '__fish_seen_subcommand_from chaos; and __fish_seen_subcommand_from status' -l json -d 'Output in JSON format'

# grpc subcommands and options
complete -c mockd -n '__fish_seen_subcommand_from grpc; and not __fish_seen_subcommand_from list call' -a 'list' -d 'List services and methods'
complete -c mockd -n '__fish_seen_subcommand_from grpc; and not __fish_seen_subcommand_from list call' -a 'call' -d 'Call a gRPC method'
complete -c mockd -n '__fish_seen_subcommand_from grpc; and __fish_seen_subcommand_from list' -s I -l import -d 'Import path for proto includes' -r -F
complete -c mockd -n '__fish_seen_subcommand_from grpc; and __fish_seen_subcommand_from call' -s m -l metadata -d 'gRPC metadata'
complete -c mockd -n '__fish_seen_subcommand_from grpc; and __fish_seen_subcommand_from call' -l plaintext -d 'Use plaintext (no TLS)'
complete -c mockd -n '__fish_seen_subcommand_from grpc; and __fish_seen_subcommand_from call' -l pretty -d 'Pretty print output'

# mqtt subcommands and options
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and not __fish_seen_subcommand_from publish subscribe status' -a 'publish' -d 'Publish a message'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and not __fish_seen_subcommand_from publish subscribe status' -a 'subscribe' -d 'Subscribe to a topic'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and not __fish_seen_subcommand_from publish subscribe status' -a 'status' -d 'Show MQTT broker status'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and __fish_seen_subcommand_from publish' -s b -l broker -d 'MQTT broker address'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and __fish_seen_subcommand_from publish' -s u -l username -d 'MQTT username'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and __fish_seen_subcommand_from publish' -s P -l password -d 'MQTT password'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and __fish_seen_subcommand_from publish' -l qos -d 'QoS level' -a '0 1 2'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and __fish_seen_subcommand_from publish' -l retain -d 'Retain message'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and __fish_seen_subcommand_from subscribe' -s b -l broker -d 'MQTT broker address'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and __fish_seen_subcommand_from subscribe' -s u -l username -d 'MQTT username'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and __fish_seen_subcommand_from subscribe' -s P -l password -d 'MQTT password'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and __fish_seen_subcommand_from subscribe' -l qos -d 'QoS level' -a '0 1 2'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and __fish_seen_subcommand_from subscribe' -s n -l count -d 'Number of messages'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and __fish_seen_subcommand_from subscribe' -s t -l timeout -d 'Timeout duration'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and __fish_seen_subcommand_from status' -l admin-url -d 'Admin API base URL'
complete -c mockd -n '__fish_seen_subcommand_from mqtt; and __fish_seen_subcommand_from status' -l json -d 'Output in JSON format'

# websocket subcommands and options
complete -c mockd -n '__fish_seen_subcommand_from websocket; and not __fish_seen_subcommand_from connect send listen status' -a 'connect' -d 'Interactive WebSocket client (REPL mode)'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and not __fish_seen_subcommand_from connect send listen status' -a 'send' -d 'Send a single message and exit'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and not __fish_seen_subcommand_from connect send listen status' -a 'listen' -d 'Stream incoming messages'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and not __fish_seen_subcommand_from connect send listen status' -a 'status' -d 'Show WebSocket handler status'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from connect' -s H -l header -d 'Custom headers (key:value)'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from connect' -l subprotocol -d 'WebSocket subprotocol'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from connect' -s t -l timeout -d 'Connection timeout'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from connect' -l json -d 'Output messages in JSON format'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from send' -s H -l header -d 'Custom headers (key:value)'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from send' -l subprotocol -d 'WebSocket subprotocol'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from send' -s t -l timeout -d 'Connection timeout'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from send' -l json -d 'Output result in JSON format'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from listen' -s H -l header -d 'Custom headers (key:value)'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from listen' -l subprotocol -d 'WebSocket subprotocol'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from listen' -s t -l timeout -d 'Connection timeout'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from listen' -s n -l count -d 'Number of messages to receive'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from listen' -l json -d 'Output messages in JSON format'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from status' -l admin-url -d 'Admin API base URL'
complete -c mockd -n '__fish_seen_subcommand_from websocket; and __fish_seen_subcommand_from status' -l json -d 'Output in JSON format'

# soap subcommands and options
complete -c mockd -n '__fish_seen_subcommand_from soap; and not __fish_seen_subcommand_from validate call' -a 'validate' -d 'Validate a WSDL file'
complete -c mockd -n '__fish_seen_subcommand_from soap; and not __fish_seen_subcommand_from validate call' -a 'call' -d 'Call a SOAP operation'
complete -c mockd -n '__fish_seen_subcommand_from soap; and __fish_seen_subcommand_from call' -s a -l action -d 'SOAPAction header'
complete -c mockd -n '__fish_seen_subcommand_from soap; and __fish_seen_subcommand_from call' -s b -l body -d 'SOAP body content'
complete -c mockd -n '__fish_seen_subcommand_from soap; and __fish_seen_subcommand_from call' -s H -l header -d 'Additional headers'
complete -c mockd -n '__fish_seen_subcommand_from soap; and __fish_seen_subcommand_from call' -l soap12 -d 'Use SOAP 1.2'
complete -c mockd -n '__fish_seen_subcommand_from soap; and __fish_seen_subcommand_from call' -l pretty -d 'Pretty print output'
complete -c mockd -n '__fish_seen_subcommand_from soap; and __fish_seen_subcommand_from call' -l timeout -d 'Request timeout'

# templates subcommands and options
complete -c mockd -n '__fish_seen_subcommand_from templates; and not __fish_seen_subcommand_from list add' -a 'list' -d 'List available templates'
complete -c mockd -n '__fish_seen_subcommand_from templates; and not __fish_seen_subcommand_from list add' -a 'add' -d 'Download and import a template'
complete -c mockd -n '__fish_seen_subcommand_from templates; and __fish_seen_subcommand_from list' -s c -l category -d 'Filter by category' -a 'protocols services patterns'
complete -c mockd -n '__fish_seen_subcommand_from templates; and __fish_seen_subcommand_from list' -l base-url -d 'Templates repository URL'
complete -c mockd -n '__fish_seen_subcommand_from templates; and __fish_seen_subcommand_from add' -s o -l output -d 'Save to file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from templates; and __fish_seen_subcommand_from add' -l admin-url -d 'Admin API base URL'
complete -c mockd -n '__fish_seen_subcommand_from templates; and __fish_seen_subcommand_from add' -l base-url -d 'Templates repository URL'
complete -c mockd -n '__fish_seen_subcommand_from templates; and __fish_seen_subcommand_from add' -l dry-run -d 'Preview without importing'

# completion options
complete -c mockd -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish' -d 'Shell type'

# version options
complete -c mockd -n '__fish_seen_subcommand_from version' -l json -d 'Output in JSON format'

# help topics
complete -c mockd -n '__fish_seen_subcommand_from help' -a 'config' -d 'Configuration file format'
complete -c mockd -n '__fish_seen_subcommand_from help' -a 'matching' -d 'Request matching patterns'
complete -c mockd -n '__fish_seen_subcommand_from help' -a 'templating' -d 'Template variable reference'
complete -c mockd -n '__fish_seen_subcommand_from help' -a 'formats' -d 'Import/export formats'
complete -c mockd -n '__fish_seen_subcommand_from help' -a 'websocket' -d 'WebSocket mock configuration'
complete -c mockd -n '__fish_seen_subcommand_from help' -a 'graphql' -d 'GraphQL mock configuration'
complete -c mockd -n '__fish_seen_subcommand_from help' -a 'grpc' -d 'gRPC mock configuration'
complete -c mockd -n '__fish_seen_subcommand_from help' -a 'mqtt' -d 'MQTT broker configuration'
complete -c mockd -n '__fish_seen_subcommand_from help' -a 'soap' -d 'SOAP/WSDL mock configuration'
complete -c mockd -n '__fish_seen_subcommand_from help' -a 'sse' -d 'Server-Sent Events configuration'
`
