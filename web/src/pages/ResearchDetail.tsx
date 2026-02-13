import React, { useEffect, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { getJob, getJobLogs } from '../api';
import ReactMarkdown from 'react-markdown';
import { ArrowLeft, Loader2, Download, Terminal, FileText } from 'lucide-react';
import clsx from 'clsx';

export const ResearchDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const logsEndRef = useRef<HTMLDivElement>(null);

  const { data: job, isLoading: jobLoading } = useQuery({
    queryKey: ['job', id],
    queryFn: () => getJob(id!),
    enabled: !!id,
    refetchInterval: (data) => (data?.status === 'completed' || data?.status === 'failed' ? false : 2000),
  });

  const { data: logs } = useQuery({
    queryKey: ['logs', id],
    queryFn: () => getJobLogs(id!),
    enabled: !!id,
    refetchInterval: (data) => (job?.status === 'completed' || job?.status === 'failed' ? false : 2000),
  });

  // Auto-scroll logs
  useEffect(() => {
    if (logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs]);

  if (jobLoading) {
    return <div className="flex justify-center p-12"><Loader2 className="w-8 h-8 animate-spin text-blue-500" /></div>;
  }

  if (!job) {
    return <div className="text-center p-12">Job not found</div>;
  }

  return (
    <div className="max-w-6xl mx-auto space-y-6">
      <div className="flex items-center gap-4 mb-6">
        <Link to="/" className="p-2 hover:bg-slate-800 rounded-full transition-colors">
          <ArrowLeft className="w-5 h-5" />
        </Link>
        <div className="flex-1">
          <h1 className="text-2xl font-bold">{job.topic}</h1>
          <div className="flex items-center gap-2 text-slate-400 text-sm mt-1">
            <span>ID: {job.id}</span>
            <span>•</span>
            <span>{new Date(job.created_at).toLocaleString()}</span>
          </div>
        </div>
        <div className={clsx(
          "px-3 py-1 rounded-full text-sm font-medium border flex items-center gap-2",
          job.status === 'running' && "bg-blue-900/30 text-blue-300 border-blue-800",
          job.status === 'completed' && "bg-green-900/30 text-green-300 border-green-800",
          job.status === 'failed' && "bg-red-900/30 text-red-300 border-red-800",
        )}>
          {job.status === 'running' && <Loader2 className="w-4 h-4 animate-spin" />}
          {job.status.toUpperCase()}
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-[600px]">
        {/* Logs Panel */}
        <div className="bg-slate-950 border border-slate-800 rounded-lg flex flex-col overflow-hidden">
          <div className="bg-slate-900 px-4 py-2 border-b border-slate-800 flex items-center gap-2">
            <Terminal className="w-4 h-4 text-slate-400" />
            <span className="font-mono text-sm text-slate-300">Execution Logs</span>
          </div>
          <div className="flex-1 overflow-y-auto p-4 font-mono text-xs space-y-1">
            {logs?.map((log) => (
              <div key={log.id} className="flex gap-2">
                <span className="text-slate-500 shrink-0">
                  {new Date(log.timestamp).toLocaleTimeString()}
                </span>
                <span className={clsx(
                  "font-bold shrink-0 w-12",
                  log.level === 'INFO' && "text-blue-400",
                  log.level === 'WARN' && "text-yellow-400",
                  log.level === 'ERROR' && "text-red-400",
                )}>
                  {log.level}
                </span>
                <span className="text-slate-300 break-words">{log.message}</span>
              </div>
            ))}
            {logs?.length === 0 && <div className="text-slate-600 italic">Waiting for logs...</div>}
            <div ref={logsEndRef} />
          </div>
        </div>

        {/* Report Panel */}
        <div className="bg-slate-900 border border-slate-800 rounded-lg flex flex-col overflow-hidden">
          <div className="bg-slate-800/50 px-4 py-2 border-b border-slate-800 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <FileText className="w-4 h-4 text-slate-400" />
              <span className="font-medium text-sm text-slate-300">Final Report</span>
            </div>
            {job.report && (
              <button 
                onClick={() => {
                  const blob = new Blob([job.report!], { type: 'text/markdown' });
                  const url = URL.createObjectURL(blob);
                  const a = document.createElement('a');
                  a.href = url;
                  a.download = `report-${job.id}.md`;
                  a.click();
                }}
                className="text-xs flex items-center gap-1 text-blue-400 hover:text-blue-300"
              >
                <Download className="w-3 h-3" />
                Download
              </button>
            )}
          </div>
          <div className="flex-1 overflow-y-auto p-6 prose prose-invert prose-sm max-w-none">
            {job.report ? (
              <ReactMarkdown>{job.report}</ReactMarkdown>
            ) : (
              <div className="h-full flex flex-col items-center justify-center text-slate-500 gap-2">
                {job.status === 'completed' ? (
                  <span>No report generated.</span>
                ) : (
                  <>
                    <Loader2 className="w-8 h-8 animate-spin opacity-20" />
                    <span>Research in progress...</span>
                  </>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
