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

Arguments:
  shell    Target shell (bash, zsh, fish)

Examples:
  # Bash
  mockd completion bash > /etc/bash_completion.d/mockd

  # Zsh
  mockd completion zsh > "${fpath[1]}/_mockd"

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

    local commands="start tunnel add list get delete import export logs config completion version help"

    if [[ ${cword} -eq 1 ]]; then
        COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
        return
    fi

    case ${words[1]} in
        start)
            COMPREPLY=($(compgen -W "--port -p --admin-port -a --config -c --https-port --read-timeout --write-timeout --max-log-entries --auto-cert --help" -- "${cur}"))
            ;;
        tunnel)
            COMPREPLY=($(compgen -W "status stop --port -p --admin-port --config -c --relay --token --subdomain -s --domain --auth-token --auth-basic --allow-ips --help" -- "${cur}"))
            ;;
        add)
            COMPREPLY=($(compgen -W "--method -m --path --status -s --body -b --body-file --header -H --match-header --match-query --name -n --priority --delay --admin-url --json --help" -- "${cur}"))
            ;;
        list)
            COMPREPLY=($(compgen -W "--admin-url --json --help" -- "${cur}"))
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
        completion)
            COMPREPLY=($(compgen -W "bash zsh fish" -- "${cur}"))
            ;;
        version)
            COMPREPLY=($(compgen -W "--json --help" -- "${cur}"))
            ;;
    esac
}

complete -F _mockd mockd
`

const zshCompletion = `#compdef mockd

_mockd() {
    local -a commands
    commands=(
        'start:Start the mock server'
        'tunnel:Expose local mocks via cloud relay'
        'add:Add a new mock endpoint'
        'list:List all configured mocks'
        'get:Get details of a specific mock'
        'delete:Delete a mock by ID'
        'import:Import mocks from a configuration file'
        'export:Export current mocks to stdout or file'
        'logs:View request logs'
        'config:Show effective configuration'
        'completion:Generate shell completion scripts'
        'version:Show version information'
        'help:Show help'
    )

    if (( CURRENT == 2 )); then
        _describe -t commands 'mockd commands' commands
        return
    fi

    case ${words[2]} in
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
                '--admin-url[Admin API base URL]:url:' \
                '--json[Output in JSON format]'
            ;;
        list|get|delete)
            _arguments \
                '--admin-url[Admin API base URL]:url:' \
                '--json[Output in JSON format]'
            ;;
        import)
            _arguments \
                '--replace[Replace all existing mocks]' \
                '--admin-url[Admin API base URL]:url:' \
                ':file:_files -g "*.json"'
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
        completion)
            _values 'shell' bash zsh fish
            ;;
        version)
            _arguments '--json[Output in JSON format]'
            ;;
    esac
}

_mockd
`

const fishCompletion = `# mockd fish completion

# Disable file completions
complete -c mockd -f

# Commands
complete -c mockd -n '__fish_use_subcommand' -a 'start' -d 'Start the mock server'
complete -c mockd -n '__fish_use_subcommand' -a 'tunnel' -d 'Expose local mocks via cloud relay'
complete -c mockd -n '__fish_use_subcommand' -a 'add' -d 'Add a new mock endpoint'
complete -c mockd -n '__fish_use_subcommand' -a 'list' -d 'List all configured mocks'
complete -c mockd -n '__fish_use_subcommand' -a 'get' -d 'Get details of a specific mock'
complete -c mockd -n '__fish_use_subcommand' -a 'delete' -d 'Delete a mock by ID'
complete -c mockd -n '__fish_use_subcommand' -a 'import' -d 'Import mocks from a configuration file'
complete -c mockd -n '__fish_use_subcommand' -a 'export' -d 'Export current mocks to stdout or file'
complete -c mockd -n '__fish_use_subcommand' -a 'logs' -d 'View request logs'
complete -c mockd -n '__fish_use_subcommand' -a 'config' -d 'Show effective configuration'
complete -c mockd -n '__fish_use_subcommand' -a 'completion' -d 'Generate shell completion scripts'
complete -c mockd -n '__fish_use_subcommand' -a 'version' -d 'Show version information'
complete -c mockd -n '__fish_use_subcommand' -a 'help' -d 'Show help'

# start options
complete -c mockd -n '__fish_seen_subcommand_from start' -s p -l port -d 'HTTP server port'
complete -c mockd -n '__fish_seen_subcommand_from start' -s a -l admin-port -d 'Admin API port'
complete -c mockd -n '__fish_seen_subcommand_from start' -s c -l config -d 'Path to mock configuration file' -r -F
complete -c mockd -n '__fish_seen_subcommand_from start' -l https-port -d 'HTTPS server port'
complete -c mockd -n '__fish_seen_subcommand_from start' -l read-timeout -d 'Read timeout in seconds'
complete -c mockd -n '__fish_seen_subcommand_from start' -l write-timeout -d 'Write timeout in seconds'
complete -c mockd -n '__fish_seen_subcommand_from start' -l max-log-entries -d 'Maximum request log entries'
complete -c mockd -n '__fish_seen_subcommand_from start' -l auto-cert -d 'Auto-generate TLS certificate'

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
complete -c mockd -n '__fish_seen_subcommand_from add' -l admin-url -d 'Admin API base URL'
complete -c mockd -n '__fish_seen_subcommand_from add' -l json -d 'Output in JSON format'

# list/get/delete options
complete -c mockd -n '__fish_seen_subcommand_from list get delete' -l admin-url -d 'Admin API base URL'
complete -c mockd -n '__fish_seen_subcommand_from list get' -l json -d 'Output in JSON format'

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

# completion options
complete -c mockd -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish' -d 'Shell type'

# version options
complete -c mockd -n '__fish_seen_subcommand_from version' -l json -d 'Output in JSON format'
`
