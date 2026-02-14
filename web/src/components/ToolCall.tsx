import React, { useState } from 'react';
import { ChevronDown, ChevronRight, Loader2, Search } from 'lucide-react';
import clsx from 'clsx';
import ReactMarkdown from 'react-markdown';

interface ToolCallProps {
  tool: {
    id: string;
    name: string;
    args: any;
    result?: any;
    status: 'pending' | 'completed' | 'error';
  };
}

export const ToolCall: React.FC<ToolCallProps> = ({ tool }) => {
  const [isOpen, setIsOpen] = useState(false);

  const getToolIcon = (name: string) => {
    switch (name) {
      case 'search_content':
        return <Search className="w-4 h-4 text-blue-400" />;
      default:
        return <Loader2 className="w-4 h-4 text-slate-400" />;
    }
  };

  const formatArgs = (args: any) => {
    if (tool.name === 'search_content' && args.query) {
      return <span className="text-slate-300 font-medium">"{args.query}"</span>;
    }
    return <span className="text-slate-400 font-mono text-xs">{JSON.stringify(args)}</span>;
  };

  return (
    <div className="border border-slate-800 rounded-lg bg-slate-900/30 overflow-hidden mb-2">
      <button 
        onClick={() => tool.status === 'completed' && setIsOpen(!isOpen)}
        disabled={tool.status !== 'completed'}
        className={clsx(
          "w-full flex items-center gap-3 px-3 py-2 text-sm text-left transition-colors",
          tool.status === 'completed' ? "hover:bg-slate-800/50 cursor-pointer" : "cursor-default"
        )}
      >
        <div className="flex items-center gap-2 flex-1 min-w-0">
          {tool.status === 'pending' ? (
            <Loader2 className="w-4 h-4 text-blue-500 animate-spin shrink-0" />
          ) : (
            getToolIcon(tool.name)
          )}
          
          <span className="font-medium text-slate-400 shrink-0">
            {tool.name === 'search_content' ? 'Searching:' : tool.name}
          </span>
          
          <div className="truncate">
            {formatArgs(tool.args)}
          </div>
        </div>

        {tool.status === 'completed' && (
          <div className="text-slate-500 shrink-0">
            {isOpen ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
          </div>
        )}
      </button>

      {isOpen && tool.result && (
        <div className="px-3 py-3 border-t border-slate-800/50 bg-slate-950/30 text-xs font-mono text-slate-400 overflow-x-auto">
             {tool.name === 'search_content' && tool.result.response?.results ? (
                 <div className="prose prose-invert prose-xs max-w-none">
                    <ReactMarkdown>{tool.result.response.results}</ReactMarkdown>
                 </div>
             ) : (
                <pre>{JSON.stringify(tool.result, null, 2)}</pre>
             )}
        </div>
      )}
    </div>
  );
};
