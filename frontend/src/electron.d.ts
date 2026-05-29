export {};

declare global {
  interface ElectronAPI {
    getServerUrl(): Promise<string>;
    setServerUrl(url: string): Promise<boolean>;
    selectProjectDir(): Promise<string | null>;
    getConnectionStatus(): Promise<string>;
    onConnectionChange(callback: (status: string) => void): () => void;
    saveConversation(id: string, title: string, messages: any[]): Promise<boolean>;
    loadConversation(id: string): Promise<any>;
    listConversations(): Promise<any[]>;
    deleteConversation(id: string): Promise<boolean>;
  }

  interface Window {
    electronAPI?: ElectronAPI;
  }
}
