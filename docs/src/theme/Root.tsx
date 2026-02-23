import React, {useState, useEffect, useCallback} from 'react';
import GitHubRepoModal from '../components/GitHubRepoModal';

const GITHUB_URL_PATTERN = 'github.com/open-pact/openpact';

export default function Root({children}: {children: React.ReactNode}) {
  const [showModal, setShowModal] = useState(false);

  const handleClick = useCallback((e: MouseEvent) => {
    const anchor = (e.target as HTMLElement).closest('a');
    if (!anchor) return;

    const href = anchor.getAttribute('href');
    if (href && href.includes(GITHUB_URL_PATTERN)) {
      e.preventDefault();
      e.stopPropagation();
      setShowModal(true);
    }
  }, []);

  useEffect(() => {
    document.addEventListener('click', handleClick, true);
    return () => document.removeEventListener('click', handleClick, true);
  }, [handleClick]);

  return (
    <>
      {children}
      {showModal && <GitHubRepoModal onClose={() => setShowModal(false)} />}
    </>
  );
}
