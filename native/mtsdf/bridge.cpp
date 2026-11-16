#define NOMINMAX
#include <windows.h>
#include <dwrite.h>
#include <d2d1.h>
#include <vector>
#include <cmath>
#include <algorithm>
#include <cstring>
#include "pipeline.h"

// ABI1: POD snapshots only. Snapshot is synchronous on the caller's UI thread;
// generate has no COM or font pointers and may run on a bounded worker.
class Sink final:public IDWriteGeometrySink {
public:
 std::vector<Segment> segments; bool bad=false; D2D1_FILL_MODE fill=D2D1_FILL_MODE_WINDING;
 HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid,void**out) override {if(!out)return E_POINTER;*out=nullptr;if(iid!=__uuidof(IUnknown)&&iid!=__uuidof(IDWriteGeometrySink))return E_NOINTERFACE;*out=this;AddRef();return S_OK;}
 ULONG STDMETHODCALLTYPE AddRef() override{return 1;} ULONG STDMETHODCALLTYPE Release() override{return 1;}
 void STDMETHODCALLTYPE SetFillMode(D2D1_FILL_MODE f) override{fill=f;}
 void STDMETHODCALLTYPE SetSegmentFlags(D2D1_PATH_SEGMENT) override{}
 void push(Segment s){if(segments.size()>=8192){bad=true;return;}segments.push_back(s);}
 void STDMETHODCALLTYPE BeginFigure(D2D1_POINT_2F p,D2D1_FIGURE_BEGIN) override{push({0,p.x,p.y,0,0,0,0});}
 void STDMETHODCALLTYPE AddLines(const D2D1_POINT_2F*p,UINT n) override{if(n>8192){bad=true;return;}for(UINT i=0;i<n;i++)push({1,p[i].x,p[i].y,0,0,0,0});}
 void STDMETHODCALLTYPE AddBeziers(const D2D1_BEZIER_SEGMENT*p,UINT n) override{if(n>8192){bad=true;return;}for(UINT i=0;i<n;i++)push({3,p[i].point1.x,p[i].point1.y,p[i].point2.x,p[i].point2.y,p[i].point3.x,p[i].point3.y});}
 void STDMETHODCALLTYPE EndFigure(D2D1_FIGURE_END end) override{if(end!=D2D1_FIGURE_END_CLOSED)bad=true;push({4,0,0,0,0,0,0});}
 HRESULT STDMETHODCALLTYPE Close() override{return bad?E_FAIL:S_OK;}
};
extern "C" __declspec(dllexport) unsigned cm_abi(){return 1;}
extern "C" __declspec(dllexport) int cm_snapshot(IDWriteFontFace*face,unsigned gid,Segment*out,unsigned capacity,unsigned*count,unsigned*fill){
 if(!face||!count||!fill||gid>65535||capacity>8192)return -1;
 try{Sink s;UINT16 g=(UINT16)gid;FLOAT advance=0;HRESULT hr=face->GetGlyphRunOutline(1.f,&g,&advance,nullptr,1,FALSE,FALSE,&s);
 if(SUCCEEDED(hr))hr=s.Close();
 if(FAILED(hr)||s.bad)return -2;*count=(unsigned)s.segments.size();*fill=(unsigned)s.fill;
 if(capacity<*count)return 1;if(*count&& !out)return -1;if(*count)memcpy(out,s.segments.data(),*count*sizeof(Segment));return 0;}catch(...){return -3;}
}
extern "C" __declspec(dllexport) unsigned cm_revision(){return 4;}
extern "C" __declspec(dllexport) int cm_generate(const Segment*data,unsigned count,unsigned fill,unsigned char*out,unsigned capacity,Result*result){
 try{return renderBoundaryMTSDF(data,count,fill,out,capacity,result);}catch(...){return -3;}
}
