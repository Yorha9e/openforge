import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react';
import type { ChangedFile } from './FileTreePanel';
import { api } from '../../shared/api';

interface LogEntry {
  type: string;
  timestamp: number;
  [key: string]: any;
}

interface FileContentData {
  content: string;
  language: string;
}

interface ProModeState {
  pipelineId: string;
  selectedFile: string | null;
  files: ChangedFile[];
  executionLogs: LogEntry[];
  isDirty: boolean;
  fileContent: FileContentData | null;
  fileContentLoading: boolean;
  selectedFileStatus: string | undefined;
  selectFile: (path: string | null) => void;
  updateFiles: (files: ChangedFile[]) => void;
  addLog: (entry: LogEntry) => void;
  clearLogs: () => void;
  setDirty: (dirty: boolean) => void;
}

const ProModeContext = createContext<ProModeState>({
  pipelineId: '',
  selectedFile: null,
  files: [],
  executionLogs: [],
  isDirty: false,
  fileContent: null,
  fileContentLoading: false,
  selectedFileStatus: undefined,
  selectFile: () => {},
  updateFiles: () => {},
  addLog: () => {},
  clearLogs: () => {},
  setDirty: () => {},
});

export function ProModeProvider({ pipelineId, children }: { pipelineId: string; children: ReactNode }) {
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [files, setFiles] = useState<ChangedFile[]>([]);
  const [executionLogs, setExecutionLogs] = useState<LogEntry[]>([]);
  const [isDirty, setIsDirty] = useState(false);
  const [fileContent, setFileContent] = useState<FileContentData | null>(null);
  const [fileContentLoading, setFileContentLoading] = useState(false);

  // Fetch file content when selectedFile changes
  useEffect(() => {
    if (!selectedFile || selectedFile.trim() === '') {
      setFileContent(null);
      return;
    }

    console.log('[ProModeContext] Fetching file content for:', selectedFile);
    setFileContentLoading(true);
    const startTime = Date.now();

    api.getFileContent(selectedFile)
      .then(data => {
        console.log('[ProModeContext] File loaded in', Date.now() - startTime, 'ms, size:', data.content.length);
        setFileContent({ content: data.content, language: data.language });
      })
      .catch(err => {
        console.error('[ProModeContext] Failed to load file content:', err);
        setFileContent({ content: `// Error loading file: ${err.message}`, language: 'plaintext' });
      })
      .finally(() => {
        setFileContentLoading(false);
      });
  }, [selectedFile]);

  const selectedFileStatus = selectedFile
    ? files.find(f => f.path === selectedFile)?.status
    : undefined;

  const selectFile = useCallback((path: string | null) => {
    setSelectedFile(path);
  }, []);

  const updateFiles = useCallback((newFiles: ChangedFile[]) => {
    setFiles(newFiles);
  }, []);

  const addLog = useCallback((entry: LogEntry) => {
    setExecutionLogs(prev => [...prev, entry]);
  }, []);

  const clearLogs = useCallback(() => {
    setExecutionLogs([]);
  }, []);

  const setDirty = useCallback((dirty: boolean) => {
    setIsDirty(dirty);
  }, []);

  return (
    <ProModeContext.Provider value={{
      pipelineId,
      selectedFile,
      files,
      executionLogs,
      isDirty,
      fileContent,
      fileContentLoading,
      selectedFileStatus,
      selectFile,
      updateFiles,
      addLog,
      clearLogs,
      setDirty,
    }}>
      {children}
    </ProModeContext.Provider>
  );
}

export function useProMode() {
  return useContext(ProModeContext);
}