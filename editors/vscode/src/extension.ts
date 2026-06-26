import * as vscode from 'vscode';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from 'vscode-languageclient/node';

let client: LanguageClient | undefined;

export function activate(context: vscode.ExtensionContext): void {
  const cfg = vscode.workspace.getConfiguration('glint');
  const glintPath = cfg.get<string>('executablePath') || 'glint';

  const serverOptions: ServerOptions = {
    command: glintPath,
    args: ['lsp'],
    transport: TransportKind.stdio,
  };

  const clientOptions: LanguageClientOptions = {
    // Only process .gitlab-ci.yml files, not all YAML.
    documentSelector: [
      { scheme: 'file', language: 'yaml', pattern: '**/.gitlab-ci.yml' },
    ],
  };

  client = new LanguageClient(
    'glint',
    'glint — GitLab CI Linter',
    serverOptions,
    clientOptions,
  );

  client.start();
  context.subscriptions.push(client);
}

export async function deactivate(): Promise<void> {
  await client?.stop();
}
