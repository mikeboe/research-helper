import React, { useState, useEffect, useRef } from 'react';
import { useParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getMessages } from '../api';
import ReactMarkdown from 'react-markdown';
import { Send, User, Bot, Loader2 } from 'lucide-react';
import clsx from 'clsx';
import { ToolCall } from '../components/ToolCall';

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8081/api';

export const Chat: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [input, setInput] = useState('');
  const bottomRef = useRef<HTMLDivElement>(null);
  const queryClient = useQueryClient();

  const [isStreaming, setIsStreaming] = useState(false);
  const [streamingContent, setStreamingContent] = useState('');
  const [streamingTools, setStreamingTools] = useState<any[]>([]);

  const { data: messages, isLoading } = useQuery({
    queryKey: ['messages', id],
    queryFn: () => getMessages(id!),
    enabled: !!id,
  });

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streamingContent, isStreaming, streamingTools]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isStreaming) return;
    
    const content = input;
    setInput('');
    setIsStreaming(true);
    setStreamingContent('');
    setStreamingTools([]);

    // Optimistic update for user message
    queryClient.setQueryData(['messages', id], (old: any[]) => [
      ...(old || []),
      { id: 'temp-user', role: 'user', content, created_at: new Date().toISOString() }
    ]);

    try {
      const response = await fetch(`${API_URL}/chat/conversations/${id}/messages`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ content }),
      });

      if (!response.ok) {
        throw new Error('Network response was not ok');
      }

      if (!response.body) {
        throw new Error('Response body is null');
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { value, done } = await reader.read();
        if (done) break;

        const chunk = decoder.decode(value, { stream: true });
        buffer += chunk;
        
        const lines = buffer.split('\n\n'); // SSE events are separated by double newline
        buffer = lines.pop() || ''; // Keep the incomplete line in the buffer

        for (const line of lines) {
          const trimmedLine = line.trim();
          if (!trimmedLine.startsWith('data: ')) continue;
          
          const jsonStr = trimmedLine.slice(6); // Remove 'data: ' prefix
          
          try {
            const event = JSON.parse(jsonStr);
            if (event.type === 'content') {
              setStreamingContent(prev => prev + event.payload);
            } else if (event.type === 'tool_call') {
               const tool = event.payload;
               setStreamingTools(prev => {
                   const existing = prev.find(t => t.id === tool.id);
                   if (existing) {
                       return prev.map(t => t.id === tool.id ? { ...t, ...tool } : t);
                   }
                   return [...prev, { ...tool, status: 'pending' }];
               });
            } else if (event.type === 'tool_result') {
               setStreamingTools(prev => prev.map(t => 
                   t.id === event.payload.id 
                       ? { ...t, result: event.payload, status: 'completed' } 
                       : t
               ));
            } else if (event.type === 'error') {
               console.error('Stream error:', event.payload);
            } else if (event.type === 'done') {
                // finished
            }
          } catch (e) {
            console.error('Error parsing JSON chunk', e);
          }
        }
      }

    } catch (error) {
      console.error('Failed to send message:', error);
    } finally {
      setIsStreaming(false);
      setStreamingContent('');
      setStreamingTools([]);
      queryClient.invalidateQueries({ queryKey: ['messages', id] });
      queryClient.invalidateQueries({ queryKey: ['conversations'] });
    }
  };

  if (isLoading && !messages) {
      return <div className="h-full flex items-center justify-center"><Loader2 className="w-8 h-8 animate-spin text-blue-500"/></div>
  }

  return (
    <div className="h-full flex flex-col max-w-4xl mx-auto w-full">
      <div className="flex-1 overflow-y-auto p-4 space-y-6">
        {messages?.length === 0 && !isStreaming && (
            <div className="flex flex-col items-center justify-center h-full text-slate-500 gap-4 opacity-50">
                <Bot className="w-16 h-16" />
                <p>Start a new conversation with the RAG agent.</p>
            </div>
        )}
        
        {messages?.map((msg) => (
          <div key={msg.id} className={clsx("flex gap-4 group", msg.role === 'user' ? "flex-row-reverse" : "flex-row")}>
            <div className={clsx(
                "w-8 h-8 rounded-full flex items-center justify-center shrink-0 mt-1",
                msg.role === 'user' ? "bg-slate-800 border border-slate-700" : "bg-blue-600"
            )}>
                {msg.role === 'user' ? <User className="w-4 h-4 text-slate-400" /> : <Bot className="w-4 h-4 text-white" />}
            </div>
            
            <div className={clsx(
                "max-w-[85%] rounded-2xl px-5 py-3.5 shadow-sm",
                msg.role === 'user' 
                    ? "bg-slate-800 text-slate-50 rounded-tr-sm" 
                    : "bg-slate-900/50 border border-slate-800 text-slate-100 rounded-tl-sm"
            )}>
                <div className="prose prose-invert prose-sm max-w-none break-words leading-relaxed">
                    <ReactMarkdown>{msg.content}</ReactMarkdown>
                </div>
            </div>
          </div>
        ))}
        
        {isStreaming && (
            <div className="flex gap-4 group flex-row">
                 <div className="w-8 h-8 rounded-full flex items-center justify-center shrink-0 mt-1 bg-blue-600">
                    <Bot className="w-4 h-4 text-white" />
                </div>
                <div className="max-w-[85%] space-y-2">
                    {/* Tool Calls */}
                    {streamingTools.length > 0 && (
                        <div className="flex flex-col gap-1 w-full max-w-2xl mb-2">
                            {streamingTools.map((tool) => (
                                <ToolCall key={tool.id} tool={tool} />
                            ))}
                        </div>
                    )}
                    
                    {/* Message Content */}
                    <div className="rounded-2xl px-5 py-3.5 shadow-sm bg-slate-900/50 border border-slate-800 text-slate-100 rounded-tl-sm">
                        {streamingContent ? (
                            <div className="prose prose-invert prose-sm max-w-none break-words leading-relaxed">
                                <ReactMarkdown>{streamingContent}</ReactMarkdown>
                            </div>
                        ) : (
                             <div className="flex items-center gap-1 text-slate-400">
                                <span className="w-1.5 h-1.5 bg-blue-400 rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
                                <span className="w-1.5 h-1.5 bg-blue-400 rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
                                <span className="w-1.5 h-1.5 bg-blue-400 rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
                            </div>
                        )}
                    </div>
                </div>
            </div>
        )}
        <div ref={bottomRef} />
      </div>

      <div className="p-4 border-t border-slate-800/50 bg-slate-950">
        <form onSubmit={handleSubmit} className="relative max-w-4xl mx-auto">
            <input
                type="text"
                value={input}
                onChange={(e) => setInput(e.target.value)}
                placeholder="Ask anything about your documents..."
                className="w-full bg-slate-900/50 border border-slate-800 text-slate-100 rounded-xl pl-5 pr-12 py-3.5 focus:outline-none focus:border-blue-500/50 focus:ring-1 focus:ring-blue-500/50 focus:bg-slate-900 transition-all placeholder:text-slate-500"
                disabled={isStreaming}
            />
            <button 
                type="submit"
                disabled={!input.trim() || isStreaming}
                className="absolute right-2 top-1/2 -translate-y-1/2 p-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg disabled:opacity-0 disabled:pointer-events-none transition-all duration-200"
            >
                <Send className="w-4 h-4" />
            </button>
        </form>
      </div>
    </div>
  );
};
