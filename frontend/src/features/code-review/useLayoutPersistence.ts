import { useState, useCallback, useEffect } from 'react';

interface DockviewSerialized {
  [key: string]: any;
}

const STORAGE_KEY_PREFIX = 'promode-layout-';

export function useLayoutPersistence(projectId: string) {
  const storageKey = `${STORAGE_KEY_PREFIX}${projectId}`;
  const [isDirty, setIsDirty] = useState(false);

  const saveLayout = useCallback((layout: DockviewSerialized) => {
    try {
      localStorage.setItem(storageKey, JSON.stringify(layout));
      setIsDirty(false);
    } catch (error) {
      console.error('Failed to save layout:', error);
    }
  }, [storageKey]);

  const loadLayout = useCallback((): any | null => {
    try {
      const stored = localStorage.getItem(storageKey);
      if (stored) {
        return JSON.parse(stored);
      }
    } catch (error) {
      console.error('Failed to load layout:', error);
    }
    return null;
  }, [storageKey]);

  const markDirty = useCallback(() => {
    setIsDirty(true);
  }, []);

  const resetLayout = useCallback(() => {
    localStorage.removeItem(storageKey);
    setIsDirty(false);
  }, [storageKey]);

  // Warn before unload if dirty
  useEffect(() => {
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (isDirty) {
        e.preventDefault();
        e.returnValue = '布局已修改，是否保存？';
        return e.returnValue;
      }
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload);
    };
  }, [isDirty]);

  return {
    saveLayout,
    loadLayout,
    markDirty,
    resetLayout,
    isDirty,
  };
}