import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';
import { getJobs, createJob } from '../api';
import { Plus, Clock, FileText, AlertCircle, CheckCircle2, Loader2 } from 'lucide-react';
import clsx from 'clsx';

export const Dashboard: React.FC = () => {
  const [topic, setTopic] = useState('');
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const { data: jobs, isLoading } = useQuery({
    queryKey: ['jobs'],
    queryFn: getJobs,
    refetchInterval: 5000,
  });

  const createMutation = useMutation({
    mutationFn: createJob,
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
      navigate(`/research/${data.id}`);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!topic.trim()) return;
    createMutation.mutate(topic);
  };

  return (
    <div className="max-w-4xl mx-auto space-y-8">
      <div className="bg-slate-900 border border-slate-800 rounded-lg p-6">
        <h2 className="text-2xl font-bold mb-4">Start New Research</h2>
        <form onSubmit={handleSubmit} className="flex gap-4">
          <input
            type="text"
            value={topic}
            onChange={(e) => setTopic(e.target.value)}
            placeholder="Enter a research topic (e.g., 'Impact of AI on Healthcare')..."
            className="flex-1 bg-slate-950 border border-slate-800 rounded-md px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
            disabled={createMutation.isPending}
          />
          <button
            type="submit"
            disabled={createMutation.isPending || !topic.trim()}
            className="bg-blue-600 hover:bg-blue-700 text-white px-6 py-2 rounded-md font-medium flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {createMutation.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />}
            Start Research
          </button>
        </form>
      </div>

      <div>
        <h2 className="text-xl font-bold mb-4">Recent Research</h2>
        {isLoading ? (
          <div className="text-center py-8 text-slate-500">Loading...</div>
        ) : (
          <div className="grid gap-4">
            {jobs?.map((job) => (
              <Link
                key={job.id}
                to={`/research/${job.id}`}
                className="block bg-slate-900 border border-slate-800 rounded-lg p-4 hover:border-blue-500/50 transition-colors"
              >
                <div className="flex items-center justify-between">
                  <h3 className="font-semibold text-lg">{job.topic}</h3>
                  <StatusBadge status={job.status} />
                </div>
                <div className="mt-2 text-sm text-slate-400 flex items-center gap-4">
                  <span className="flex items-center gap-1">
                    <Clock className="w-4 h-4" />
                    {new Date(job.created_at).toLocaleDateString()}
                  </span>
                  {job.status === 'completed' && (
                    <span className="flex items-center gap-1 text-green-400">
                      <FileText className="w-4 h-4" />
                      Report Ready
                    </span>
                  )}
                </div>
              </Link>
            ))}
            {jobs?.length === 0 && (
              <div className="text-center py-12 text-slate-500 border border-dashed border-slate-800 rounded-lg">
                No research jobs found. Start one above!
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
};

const StatusBadge: React.FC<{ status: string }> = ({ status }) => {
  const styles = {
    pending: 'bg-slate-800 text-slate-300',
    running: 'bg-blue-900/50 text-blue-200 border-blue-800',
    completed: 'bg-green-900/50 text-green-200 border-green-800',
    failed: 'bg-red-900/50 text-red-200 border-red-800',
  };

  const icons = {
    pending: Clock,
    running: Loader2,
    completed: CheckCircle2,
    failed: AlertCircle,
  };

  const Icon = icons[status as keyof typeof icons] || Clock;

  return (
    <span
      className={clsx(
        'px-2.5 py-0.5 rounded-full text-xs font-medium border flex items-center gap-1.5',
        styles[status as keyof typeof styles] || styles.pending
      )}
    >
      <Icon className={clsx('w-3 h-3', status === 'running' && 'animate-spin')} />
      {status.toUpperCase()}
    </span>
  );
};
