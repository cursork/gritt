const vscode = require('vscode');
const path = require('path');

function activate(context) {
    const factory = {
        createDebugAdapterDescriptor(session) {
            const adapterPath = path.join(context.extensionPath, 'apldap');
            return new vscode.DebugAdapterExecutable(adapterPath);
        }
    };

    context.subscriptions.push(
        vscode.debug.registerDebugAdapterDescriptorFactory('apl', factory)
    );
}

function deactivate() {}

module.exports = { activate, deactivate };
