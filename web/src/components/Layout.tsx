import React from 'react';
import { Link, NavLink, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getConversations, createConversation } from '../api';
import { Microscope, Plus, Menu } from 'lucide-react';
import clsx from 'clsx';

export const Layout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [isSidebarOpen, setSidebarOpen] = React.useState(true);

  const { data: conversations } = useQuery({
    queryKey: ['conversations'],
    queryFn: getConversations,
  });

  const createMutation = useMutation({
    mutationFn: createConversation,
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['conversations'] });
      navigate(`/c/${data.id}`);
    },
  });

  return (
    <div className="h-screen flex flex-col overflow-hidden bg-slate-950 text-slate-50">
      {/* Header */}
      <header className="border-b border-slate-800 bg-slate-900/50 backdrop-blur-sm shrink-0 z-10">
        <div className="px-4 h-14 flex items-center justify-between">
            <div className="flex items-center gap-4">
                <button onClick={() => setSidebarOpen(!isSidebarOpen)} className="p-2 hover:bg-slate-800 rounded-md text-slate-400 hover:text-white">
                    <Menu className="w-5 h-5" />
                </button>
                <Link to="/" className="flex items-center gap-2 font-bold text-lg text-blue-400">
                    <Microscope className="w-5 h-5" />
                    ResearchAgent
                </Link>
            </div>
        </div>
      </header>

      <div className="flex-1 flex overflow-hidden">
        {/* Sidebar */}
        <aside className={clsx(
            "w-64 border-r border-slate-800 bg-slate-900/30 flex flex-col transition-all duration-300 shrink-0",
            !isSidebarOpen && "-ml-64"
        )}>
            <div className="p-3">
                <button 
                    onClick={() => createMutation.mutate()}
                    className="w-full flex items-center justify-center gap-2 bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-md font-medium transition-colors text-sm"
                >
                    <Plus className="w-4 h-4" />
                    New Chat
                </button>
            </div>
            
            <div className="flex-1 overflow-y-auto px-2 py-2 space-y-1">
                 <NavLink 
                    to="/" 
                    end
                    className={({ isActive }) => clsx(
                        "flex items-center gap-2 px-3 py-2 text-sm rounded-md transition-colors",
                        isActive ? "bg-slate-800 text-white" : "text-slate-400 hover:text-slate-100 hover:bg-slate-900"
                    )}
                >
                    <Microscope className="w-4 h-4" />
                    Research Dashboard
                </NavLink>
                
                <div className="px-3 py-2 text-xs font-semibold text-slate-500 uppercase mt-4">Conversations</div>
                
                {conversations?.map(conv => (
                    <NavLink
                        key={conv.id}
                        to={`/c/${conv.id}`}
                        className={({ isActive }) => clsx(
                            "block px-3 py-2 text-sm rounded-md truncate transition-colors",
                            isActive ? "bg-slate-800 text-white" : "text-slate-400 hover:text-slate-100 hover:bg-slate-900"
                        )}
                    >
                        {conv.title}
                    </NavLink>
                ))}
            </div>
        </aside>

        {/* Main Content */}
        <main className="flex-1 overflow-hidden relative flex flex-col">
            {children}
        </main>
      </div>
    </div>
  );
};
